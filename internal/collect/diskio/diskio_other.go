//go:build !linux

package diskio

import (
	"context"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	psdisk "github.com/shirou/gopsutil/v4/disk"
)

type reading struct {
	read, write, io uint64
	at              time.Time
}
type Collector struct {
	mu   sync.Mutex
	last map[string]reading
}

func New(_ string, _ string) (*Collector, error) { return &Collector{last: map[string]reading{}}, nil }

func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	stats, err := psdisk.IOCountersWithContext(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var samples []metric.Sample
	var totalRead, totalWrite float64
	for name, stat := range stats {
		current := reading{stat.ReadBytes, stat.WriteBytes, stat.IoTime, now}
		previous, ok := c.last[name]
		c.last[name] = current
		if !ok {
			continue
		}
		seconds := now.Sub(previous.at).Seconds()
		if seconds <= 0 || stat.ReadBytes < previous.read || stat.WriteBytes < previous.write {
			continue
		}
		read := float64(stat.ReadBytes-previous.read) / seconds
		write := float64(stat.WriteBytes-previous.write) / seconds
		totalRead += read
		totalWrite += write
		samples = append(samples, metric.Sample{Key: series.Key("diskio." + name + ".read"), Value: read, Time: now}, metric.Sample{Key: series.Key("diskio." + name + ".write"), Value: write, Time: now})
		if stat.IoTime >= previous.io {
			samples = append(samples, metric.Sample{Key: series.Key("diskio." + name + ".util"), Value: float64(stat.IoTime-previous.io) / (seconds * 10), Time: now})
		}
	}
	samples = append(samples, metric.Sample{Key: "diskio.total.read", Value: totalRead, Time: now}, metric.Sample{Key: "diskio.total.write", Value: totalWrite, Time: now})
	return samples, nil
}
