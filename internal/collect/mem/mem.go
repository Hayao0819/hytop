//go:build linux

// Package mem reads /proc/meminfo.
package mem

import (
	"context"
	"time"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct{ fs procfs.FS }

func New(root string) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}

	return &Collector{fs: fs}, nil
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.Meminfo()

	return collect.KernelAvailability(err, "mount /proc")
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	info, err := c.fs.Meminfo()
	if err != nil {
		return nil, errors.Wrap(err, "reading /proc/meminfo")
	}

	kib := func(v *uint64) float64 {
		if v == nil {
			return 0
		}

		return float64(*v) * 1024
	}

	var (
		total     = kib(info.MemTotal)
		available = kib(info.MemAvailable)
		used      = total - available
		swapTotal = kib(info.SwapTotal)
	)

	samples := []metric.Sample{
		{Key: "mem.total", Value: total, Time: now},
		{Key: "mem.used", Value: used, Time: now},
		{Key: "mem.available", Value: available, Time: now},
		{Key: "mem.cached", Value: kib(info.Cached), Time: now},
		{Key: "swap.total", Value: swapTotal, Time: now},
		{Key: "swap.used", Value: swapTotal - kib(info.SwapFree), Time: now},
	}

	if total > 0 {
		samples = append(samples, metric.Sample{Key: "mem.usage", Value: 100 * used / total, Time: now})
	}

	return samples, nil
}
