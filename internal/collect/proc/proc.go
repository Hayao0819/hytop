//go:build linux

// Package proc reads /proc/[pid] into process snapshots.
package proc

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/errors"
)

// kthreaddPID is where every kernel thread comes from.
const kthreaddPID = 2

type Collector struct {
	fs   procfs.FS
	root string
	self int
	read [4096]byte

	clockTicks float64
	pageSize   uint64

	last  time.Time
	ticks map[int]cpuMark

	// ownTicks and ownAt form the baseline for hytop's CPU rate.
	ownTicks float64
	ownAt    time.Time
	users    map[int64]string

	// Processes populates tally for the next Collect; Prime runs twice to initialize it.
	tally tally
}

type tally struct {
	total    int
	running  int
	sleeping int
	stopped  int
	zombie   int
	threads  int
	fds      int
}

func New(root string, self int) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}

	return &Collector{
		fs:         fs,
		root:       root,
		self:       self,
		clockTicks: 100,
		pageSize:   uint64(os.Getpagesize()),
		ticks:      make(map[int]cpuMark),
		users:      make(map[int64]string),
	}, nil
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.AllProcs()

	return collect.KernelAvailability(err, "mount /proc")
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	self, err := c.fs.Proc(c.self)
	if err != nil {
		return nil, errors.Wrapf(err, "reading /proc/%d", c.self)
	}

	stat, err := self.Stat()
	if err != nil {
		return nil, errors.Wrapf(err, "reading /proc/%d/stat", c.self)
	}

	samples := []metric.Sample{
		{Key: "self.cpu", Value: c.ownCPU(stat, now), Time: now},
		{Key: "self.rss", Value: float64(uint64(stat.RSS) * c.pageSize), Time: now},
		{Key: "proc.count", Value: float64(c.tally.total), Time: now},
		{Key: "proc.running", Value: float64(c.tally.running), Time: now},
		{Key: "proc.sleeping", Value: float64(c.tally.sleeping), Time: now},
		{Key: "proc.stopped", Value: float64(c.tally.stopped), Time: now},
		{Key: "proc.zombie", Value: float64(c.tally.zombie), Time: now},
		{Key: "proc.threads", Value: float64(c.tally.threads), Time: now},
		{Key: "proc.fds", Value: float64(c.tally.fds), Time: now},
	}

	// Load averages are optional; retain the process metrics if unavailable.
	if load, err := c.fs.LoadAvg(); err == nil {
		samples = append(samples,
			metric.Sample{Key: "load.1", Value: load.Load1, Time: now},
			metric.Sample{Key: "load.5", Value: load.Load5, Time: now},
			metric.Sample{Key: "load.15", Value: load.Load15, Time: now},
		)
	}

	return samples, nil
}

// ownCPU returns hytop's CPU use as a percentage of one core.
func (c *Collector) ownCPU(stat procfs.ProcStat, now time.Time) float64 {
	used := float64(stat.UTime+stat.STime) / c.clockTicks

	elapsed := now.Sub(c.ownAt).Seconds()
	previous := c.ownTicks

	c.ownTicks, c.ownAt = used, now

	if previous == 0 || elapsed <= 0 {
		return 0
	}

	return 100 * (used - previous) / elapsed
}

// Processes reports CPU as a share of one core over the interval since the last
// call, so the first call reports zero.
func (c *Collector) Processes(ctx context.Context) ([]procmodel.Process, error) {
	all, err := c.fs.AllProcs()
	if err != nil {
		return nil, errors.Wrap(err, "listing /proc")
	}

	var (
		now     = time.Now()
		elapsed = now.Sub(c.last).Seconds()
		procs   = make([]procmodel.Process, 0, len(all))
		seen    = make(map[int]cpuMark, len(all))
		counted tally
	)

	for _, p := range all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		stat, err := p.Stat()
		if err != nil {
			// A process that exited between the listing and the read is normal.
			continue
		}

		current := cpuMark{
			started: stat.Starttime,
			total:   float64(stat.UTime+stat.STime) / c.clockTicks,
		}
		previous, known := c.ticks[p.PID]
		seen[p.PID] = current
		cpu := cpuRate(previous.total, current.total, elapsed, known && previous.started == current.started)

		proc := c.build(p, stat, cpu)

		counted.total++
		counted.threads += proc.Threads
		counted.fds += proc.FDs

		// Derive all states from one scan so their counts remain consistent.
		switch proc.State {
		case "R":
			counted.running++
		case "S", "D", "I":
			counted.sleeping++
		case "T", "t":
			counted.stopped++
		case "Z":
			counted.zombie++
		}

		procs = append(procs, proc)
	}

	c.ticks, c.last, c.tally = seen, now, counted

	return procs, nil
}

func (c *Collector) build(p procfs.Proc, stat procfs.ProcStat, cpu float64) procmodel.Process {
	proc := procmodel.Process{
		PID:     p.PID,
		PPID:    stat.PPID,
		Started: stat.Starttime,
		Name:    safe.Text(stat.Comm),
		State:   stat.State,
		Nice:    int(stat.Nice),
		Threads: stat.NumThreads,
		CPU:     cpu,
		RSS:     uint64(stat.RSS) * c.pageSize,
		VSZ:     uint64(stat.VSize),
	}

	if cmdline, err := p.CmdLine(); err == nil {
		proc.Cmdline = safe.Text(strings.Join(cmdline, " "))

		// Zombies also have no command line, so ancestry distinguishes them from kernel threads.
		proc.Kthread = len(cmdline) == 0 && (p.PID == kthreaddPID || stat.PPID == kthreaddPID)
	}

	if exe, err := p.Executable(); err == nil {
		proc.Exe = safe.Text(exe)
	}

	if value, ok := readStatusUID(c.procFile(p.PID, "status"), c.read[:]); ok {
		proc.UID = uid(value)
		proc.User = c.username(proc.UID)
	}

	if fds, err := p.FileDescriptorsLen(); err == nil {
		proc.FDs = fds
	}

	if io, err := p.IO(); err == nil {
		proc.IORd, proc.IOWr = io.ReadBytes, io.WriteBytes
	}

	if contents, err := readSmallFile(c.procFile(p.PID, "cgroup"), c.read[:]); err == nil {
		proc.Cgroup, proc.Runtime, proc.Container = cgroupInfo(contents)
		proc.Unit = unitOf(proc.Cgroup)
	}

	return proc
}

func (c *Collector) procFile(pid int, name string) string {
	return filepath.Join(c.root, strconv.Itoa(pid), name)
}

func readStatusUID(path string, scratch []byte) (uint64, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer func() { _ = file.Close() }()

	read := 0
	for read < len(scratch) {
		n, readErr := file.Read(scratch[read:])
		read += n
		if value, ok := statusUID(scratch[:read]); ok {
			return value, true
		}
		if readErr != nil || n == 0 {
			return 0, false
		}
	}

	return 0, false
}

func readSmallFile(path string, scratch []byte) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	n, err := io.ReadFull(file, scratch)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return scratch[:n], nil
	}
	if err != nil {
		return nil, err
	}

	rest, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	contents := make([]byte, n+len(rest))
	copy(contents, scratch[:n])
	copy(contents[n:], rest)

	return contents, nil
}

func statusUID(contents []byte) (uint64, bool) {
	for line := range bytes.SplitSeq(contents, []byte{'\n'}) {
		value, ok := bytes.CutPrefix(line, []byte("Uid:"))
		if !ok {
			continue
		}

		start := 0
		for start < len(value) && (value[start] == ' ' || value[start] == '\t') {
			start++
		}
		end := start
		for end < len(value) && value[end] >= '0' && value[end] <= '9' {
			end++
		}
		if start == end {
			return 0, false
		}

		parsed, err := strconv.ParseUint(string(value[start:end]), 10, 64)

		return parsed, err == nil
	}

	return 0, false
}

func (c *Collector) username(uid int64) string {
	if name, ok := c.users[uid]; ok {
		return name
	}

	name := strconv.FormatInt(uid, 10)
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}

	c.users[uid] = name

	return name
}

func uid(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}

	return int64(value)
}

func unitOf(cgroup string) string {
	for _, part := range strings.Split(cgroup, "/") {
		if strings.HasSuffix(part, ".service") || strings.HasSuffix(part, ".scope") {
			return part
		}
	}

	return ""
}
