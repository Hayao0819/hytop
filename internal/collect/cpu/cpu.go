//go:build linux

// Package cpu reads /proc/stat.
package cpu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	fs      procfs.FS
	sysRoot string

	previous map[string]procfs.CPUStat
	sensor   string
}

func New(root, sysRoot string) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}

	c := &Collector{fs: fs, sysRoot: sysRoot, previous: make(map[string]procfs.CPUStat)}
	c.sensor = c.findSensor()

	return c, nil
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.Stat()

	return collect.KernelAvailability(err, "mount /proc")
}

// Collect reports nothing on the first round: usage is a difference between two
// readings, and there is no earlier one yet.
func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	stat, err := c.fs.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "reading /proc/stat")
	}

	samples := make([]metric.Sample, 0, len(stat.CPU)+2)

	if usage, ok := c.usage("total", stat.CPUTotal); ok {
		samples = append(samples, metric.Sample{Key: "cpu.total.usage", Value: usage, Time: now})
	}

	for n, core := range stat.CPU {
		id := fmt.Sprint(n)

		if usage, ok := c.usage(id, core); ok {
			samples = append(samples, metric.Sample{
				Key:   series.Key("cpu.core." + id + ".usage"),
				Value: usage,
				Time:  now,
			})
		}
	}

	samples = append(samples,
		metric.Sample{Key: "proc.blocked", Value: float64(stat.ProcessesBlocked), Time: now},
	)

	samples = append(samples, c.clocks(now)...)

	if temp, ok := c.temperature(); ok {
		samples = append(samples, metric.Sample{Key: "cpu.package.temp", Value: temp, Time: now})
	}

	if boot, err := c.fs.Stat(); err == nil && boot.BootTime > 0 {
		samples = append(samples, metric.Sample{
			Key:   "system.uptime",
			Value: now.Sub(time.Unix(int64(boot.BootTime), 0)).Seconds(),
			Time:  now,
		})
	}

	return samples, nil
}

// clocks prefers cpufreq, which is per core and current. /proc/cpuinfo reports
// the same thing on many machines but not all, so it is the fallback.
func (c *Collector) clocks(now time.Time) []metric.Sample {
	var (
		samples []metric.Sample
		sum     float64
		count   int
	)

	// cpufreq directory indices can contain gaps, so iterate the kernel's list.
	for _, n := range c.present() {
		id := fmt.Sprint(n)

		kHz, ok := sysread.Float(filepath.Join(c.sysRoot, "devices", "system", "cpu", "cpu"+id, "cpufreq", "scaling_cur_freq"))
		if !ok {
			continue
		}

		hz := kHz * 1000
		sum += hz
		count++

		samples = append(samples, metric.Sample{
			Key:   series.Key("cpu.core." + id + ".freq"),
			Value: hz,
			Time:  now,
		})
	}

	if count == 0 {
		return c.clocksFromCPUInfo(now)
	}

	return append(samples, metric.Sample{Key: "cpu.total.freq", Value: sum / float64(count), Time: now})
}

// present parses the kernel's stable CPU topology list, including offline CPUs.
func (c *Collector) present() []int {
	raw, err := os.ReadFile(filepath.Join(c.sysRoot, "devices", "system", "cpu", "present"))
	if err != nil {
		return nil
	}

	var found []int

	for _, part := range strings.Split(strings.TrimSpace(string(raw)), ",") {
		low, high, ranged := strings.Cut(part, "-")

		first, err := strconv.Atoi(low)
		if err != nil {
			continue
		}

		last := first

		if ranged {
			if last, err = strconv.Atoi(high); err != nil {
				continue
			}
		}

		for n := first; n <= last; n++ {
			found = append(found, n)
		}
	}

	return found
}

func (c *Collector) clocksFromCPUInfo(now time.Time) []metric.Sample {
	info, err := c.fs.CPUInfo()
	if err != nil {
		return nil
	}

	var (
		samples = make([]metric.Sample, 0, len(info)+1)
		sum     float64
	)

	for n, cpu := range info {
		hz := cpu.CPUMHz * 1e6
		sum += hz

		samples = append(samples, metric.Sample{
			Key:   series.Key("cpu.core." + fmt.Sprint(n) + ".freq"),
			Value: hz,
			Time:  now,
		})
	}

	if len(info) == 0 {
		return nil
	}

	return append(samples, metric.Sample{Key: "cpu.total.freq", Value: sum / float64(len(info)), Time: now})
}

// findSensor picks the hwmon device that reports the package temperature.
// coretemp is Intel, k10temp is AMD; anything else is left to the thermal page.
func (c *Collector) findSensor() string {
	root := filepath.Join(c.sysRoot, "class", "hwmon")

	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		switch sysread.Text(filepath.Join(root, entry.Name(), "name")) {
		case "coretemp", "k10temp", "zenpower":
			return filepath.Join(root, entry.Name())
		}
	}

	return ""
}

func (c *Collector) temperature() (float64, bool) {
	if c.sensor == "" {
		return 0, false
	}

	milli, ok := sysread.Float(filepath.Join(c.sensor, "temp1_input"))
	if !ok {
		return 0, false
	}

	return milli / 1000, true
}

func (c *Collector) usage(id string, current procfs.CPUStat) (float64, bool) {
	previous, seen := c.previous[id]
	c.previous[id] = current

	if !seen {
		return 0, false
	}

	busy := (current.User - previous.User) +
		(current.Nice - previous.Nice) +
		(current.System - previous.System) +
		(current.IRQ - previous.IRQ) +
		(current.SoftIRQ - previous.SoftIRQ) +
		(current.Steal - previous.Steal)

	total := busy +
		(current.Idle - previous.Idle) +
		(current.Iowait - previous.Iowait)

	if total <= 0 {
		return 0, false
	}

	return 100 * busy / total, true
}
