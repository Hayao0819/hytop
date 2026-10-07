//go:build linux

// Package mem reads /proc/meminfo.
package mem

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	fs      procfs.FS
	sysRoot string
}

func New(root string, sysRoots ...string) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}

	sysRoot := "/sys"
	if len(sysRoots) > 0 {
		sysRoot = sysRoots[0]
	}

	return &Collector{fs: fs, sysRoot: sysRoot}, nil
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

	if info.Zswap != nil {
		samples = append(samples, metric.Sample{Key: "mem.zswap.compressed", Value: kib(info.Zswap), Time: now})
	}
	if info.Zswapped != nil {
		samples = append(samples, metric.Sample{Key: "mem.zswap.stored", Value: kib(info.Zswapped), Time: now})
	}

	if original, compressed, memory, ok := c.zram(); ok {
		samples = append(samples,
			metric.Sample{Key: "mem.zram.original", Value: original, Time: now},
			metric.Sample{Key: "mem.zram.compressed", Value: compressed, Time: now},
			metric.Sample{Key: "mem.zram.used", Value: memory, Time: now},
		)
		if compressed > 0 {
			samples = append(samples, metric.Sample{Key: "mem.zram.ratio", Value: original / compressed, Time: now})
		}
	}

	return samples, nil
}

func (c *Collector) zram() (original, compressed, memory float64, ok bool) {
	entries, err := os.ReadDir(filepath.Join(c.sysRoot, "block"))
	if err != nil {
		return 0, 0, 0, false
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "zram") {
			continue
		}
		value, found := sysread.String(filepath.Join(c.sysRoot, "block", entry.Name(), "mm_stat"))
		if !found {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) < 3 {
			continue
		}

		values := [3]uint64{}
		valid := true
		for i := range values {
			values[i], err = strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				valid = false
				break
			}
		}
		if valid {
			original += float64(values[0])
			compressed += float64(values[1])
			memory += float64(values[2])
			ok = true
		}
	}

	return original, compressed, memory, ok
}
