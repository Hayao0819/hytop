//go:build !linux

package filesystem

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	psdisk "github.com/shirou/gopsutil/v4/disk"
)

type Collector struct {
	mu     sync.RWMutex
	mounts []diskmodel.Mount
}

func mountFromStats(part psdisk.PartitionStat, usage *psdisk.UsageStat) diskmodel.Mount {
	return diskmodel.Mount{
		Path:       part.Mountpoint,
		Device:     part.Device,
		FSType:     part.Fstype,
		Opts:       strings.Join(part.Opts, ","),
		Total:      usage.Total,
		Free:       usage.Total - usage.Used,
		Available:  usage.Free,
		Inodes:     usage.InodesTotal,
		InodesFree: usage.InodesFree,
	}
}

func New(string) *Collector                    { return &Collector{} }
func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }
func (c *Collector) Mounts() []diskmodel.Mount {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.mounts)
}

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	parts, err := psdisk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}
	mounts := make([]diskmodel.Mount, 0, len(parts))
	for _, part := range parts {
		usage, err := psdisk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil {
			continue
		}
		mounts = append(mounts, mountFromStats(part, usage))
	}
	c.mu.Lock()
	c.mounts = mounts
	c.mu.Unlock()
	return buildSamples(mounts, now, func(diskmodel.Mount) bool { return true }), nil
}

// Boundaries includes pseudo and hidden partitions as well as the filesystems
// shown in the capacity table.
func Boundaries(string) ([]diskmodel.Mount, error) {
	parts, err := psdisk.Partitions(true)
	if err != nil {
		return nil, err
	}

	mounts := make([]diskmodel.Mount, 0, len(parts))
	for _, part := range parts {
		mounts = append(mounts, diskmodel.Mount{Path: part.Mountpoint})
	}

	return mounts, nil
}
