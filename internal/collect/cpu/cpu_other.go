//go:build !linux

package cpu

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	pscpu "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	mu   sync.Mutex
	last map[string]pscpu.TimesStat
}

func New(_ string, _ string) (*Collector, error) {
	return &Collector{last: map[string]pscpu.TimesStat{}}, nil
}
func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	times, err := pscpu.TimesWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	total, err := pscpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, err
	}
	times = append(total, times...)

	c.mu.Lock()
	defer c.mu.Unlock()
	samples := make([]metric.Sample, 0, len(times)+2)
	for i, current := range times {
		name := current.CPU
		key := series.Key("cpu.total.usage")
		if i > 0 {
			key = series.Key("cpu.core." + strconv.Itoa(i-1) + ".usage")
		}
		if previous, ok := c.last[name]; ok {
			elapsed := current.Total() - previous.Total()
			idle := current.Idle - previous.Idle
			if elapsed > 0 {
				samples = append(samples, metric.Sample{Key: key, Value: 100 * (elapsed - idle) / elapsed, Time: now})
			}
		}
		c.last[name] = current
	}
	if uptime, err := host.UptimeWithContext(ctx); err == nil {
		samples = append(samples, metric.Sample{Key: "system.uptime", Value: float64(uptime), Time: now})
	}
	return samples, nil
}

func (*Collector) Facts(ctx context.Context) (collect.Facts, error) {
	info, err := pscpu.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	logical, err := pscpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	cores, err := pscpu.CountsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	return portableFacts(info, logical, cores)
}

func portableFacts(info []pscpu.InfoStat, logical, cores int) (collect.Facts, error) {
	if len(info) == 0 {
		return nil, errors.New("the operating system listed no processors")
	}

	sockets := make(map[string]bool)
	for _, item := range info {
		if item.PhysicalID != "" {
			sockets[item.PhysicalID] = true
		}
	}

	return collect.Facts{
		series.FactCPUModel:     info[0].ModelName,
		series.FactCPUVendor:    info[0].VendorID,
		series.FactCPULogical:   strconv.Itoa(max(logical, 1)),
		series.FactCPUCores:     strconv.Itoa(max(cores, 1)),
		series.FactCPUSockets:   strconv.Itoa(max(len(sockets), 1)),
		series.FactCPUBaseSpeed: fmt.Sprintf("%.0f MHz", info[0].Mhz),
	}, nil
}
