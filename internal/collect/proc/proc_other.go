//go:build !linux

package proc

import (
	"context"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	psproc "github.com/shirou/gopsutil/v4/process"
)

type Collector struct {
	mu        sync.Mutex
	processes []procmodel.Process
	self      int
	last      time.Time
	cpu       map[int32]cpuMark
}

func New(_ string, self int) (*Collector, error) {
	return &Collector{self: self, cpu: make(map[int32]cpuMark)}, nil
}
func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	procs, err := c.read(ctx, now)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.processes = procs
	c.mu.Unlock()
	var running, sleeping, stopped, zombie, threads, fds int
	var selfCPU float64
	var selfRSS uint64
	for _, p := range procs {
		threads += p.Threads
		fds += p.FDs
		switch p.State {
		case "R", "running":
			running++
		case "S", "I", "sleeping", "idle":
			sleeping++
		case "T", "stopped":
			stopped++
		case "Z", "zombie":
			zombie++
		}
		if p.PID == c.self {
			selfCPU = p.CPU
			selfRSS = p.RSS
		}
	}
	return []metric.Sample{{Key: "proc.count", Value: float64(len(procs)), Time: now}, {Key: "proc.running", Value: float64(running), Time: now}, {Key: "proc.sleeping", Value: float64(sleeping), Time: now}, {Key: "proc.stopped", Value: float64(stopped), Time: now}, {Key: "proc.zombie", Value: float64(zombie), Time: now}, {Key: "proc.threads", Value: float64(threads), Time: now}, {Key: "proc.fds", Value: float64(fds), Time: now}, {Key: "self.cpu", Value: selfCPU, Time: now}, {Key: "self.rss", Value: float64(selfRSS), Time: now}}, nil
}

func (c *Collector) Processes(context.Context) ([]procmodel.Process, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.processes, nil
}

func (c *Collector) read(ctx context.Context, now time.Time) ([]procmodel.Process, error) {
	items, err := psproc.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]procmodel.Process, 0, len(items))
	nextCPU := make(map[int32]cpuMark, len(items))
	elapsed := now.Sub(c.last).Seconds()
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		p := procmodel.Process{PID: int(item.Pid), UID: -1}
		if value, e := item.PpidWithContext(ctx); e == nil {
			p.PPID = int(value)
		}
		if value, e := item.NameWithContext(ctx); e == nil {
			p.Name = safe.Text(value)
		}
		if value, e := item.CmdlineWithContext(ctx); e == nil {
			p.Cmdline = safe.Text(value)
		}
		if value, e := item.ExeWithContext(ctx); e == nil {
			p.Exe = safe.Text(value)
		}
		if value, e := item.UsernameWithContext(ctx); e == nil {
			p.User = safe.Text(value)
		}
		if value, e := item.UidsWithContext(ctx); e == nil && len(value) > 0 {
			p.UID = int64(value[0])
		}
		if value, e := item.StatusWithContext(ctx); e == nil && len(value) > 0 {
			p.State = safe.Text(value[0])
		}
		if value, e := item.NiceWithContext(ctx); e == nil {
			p.Nice = int(value)
		}
		if value, e := item.NumThreadsWithContext(ctx); e == nil {
			p.Threads = int(value)
		}
		if value, e := item.NumFDsWithContext(ctx); e == nil {
			p.FDs = int(value)
		}
		created, createdErr := item.CreateTimeWithContext(ctx)
		if createdErr == nil {
			p.Started = uint64(created)
		}
		if value, e := item.TimesWithContext(ctx); e == nil {
			current := cpuMark{started: p.Started, total: value.Total()}
			previous, known := c.cpu[item.Pid]
			p.CPU = cpuRate(previous.total, current.total, elapsed,
				known && current.started != 0 && previous.started == current.started)
			nextCPU[item.Pid] = current
		}
		if value, e := item.MemoryInfoWithContext(ctx); e == nil {
			p.RSS = value.RSS
			p.VSZ = value.VMS
		}
		if value, e := item.IOCountersWithContext(ctx); e == nil {
			p.IORd = value.ReadBytes
			p.IOWr = value.WriteBytes
		}
		out = append(out, p)
	}
	c.cpu, c.last = nextCPU, now
	return out, nil
}
