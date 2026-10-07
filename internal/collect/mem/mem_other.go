//go:build !linux

package mem

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	psmem "github.com/shirou/gopsutil/v4/mem"
)

type Collector struct{}

func New(string) (*Collector, error)           { return &Collector{}, nil }
func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }
func (*Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	v, err := psmem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	s, err := psmem.SwapMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return []metric.Sample{
		{Key: "mem.total", Value: float64(v.Total), Time: now},
		{Key: "mem.used", Value: float64(v.Used), Time: now},
		{Key: "mem.available", Value: float64(v.Available), Time: now},
		{Key: "mem.cached", Value: float64(v.Cached), Time: now},
		{Key: "mem.usage", Value: v.UsedPercent, Time: now},
		{Key: "swap.total", Value: float64(s.Total), Time: now},
		{Key: "swap.used", Value: float64(s.Used), Time: now},
	}, nil
}
