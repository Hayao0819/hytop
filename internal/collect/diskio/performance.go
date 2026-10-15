package diskio

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

// WMI raw rate counters contain cumulative bytes; idle time and timestamps use 100 ns ticks.
type performanceCounters struct {
	Name                 string
	DiskReadBytesPerSec  uint64
	DiskWriteBytesPerSec uint64
	PercentIdleTime      uint64
	PercentIdleTime_Base uint64
	Timestamp_Sys100NS   uint64
}

type performanceResult struct {
	counters []performanceCounters
	err      error
}

type performanceQuery struct {
	ctx  context.Context
	done <-chan performanceResult
}

type performanceCollector struct {
	gate     chan struct{}
	query    func() ([]performanceCounters, error)
	pending  *performanceQuery
	previous map[string]performanceCounters
}

func newPerformanceCollector(query func() ([]performanceCounters, error)) *performanceCollector {
	return &performanceCollector{gate: make(chan struct{}, 1), query: query}
}

func (c *performanceCollector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.gate <- struct{}{}:
	}
	defer func() { <-c.gate }()

	counters, err := c.read(ctx)
	if err != nil {
		return nil, err
	}

	samples, previous := performanceSamples(c.previous, counters, now)
	c.previous = previous

	return samples, nil
}

func (c *performanceCollector) read(ctx context.Context) ([]performanceCounters, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if c.pending == nil {
			done := make(chan performanceResult, 1)
			c.pending = &performanceQuery{ctx: ctx, done: done}
			go func() {
				counters, err := c.query()
				done <- performanceResult{counters: counters, err: err}
			}()
		}

		pending := c.pending
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-pending.done:
			c.pending = nil
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if pending.ctx.Err() != nil {
				continue
			}
			if result.err != nil {
				return nil, errors.Wrap(result.err, "reading physical disk performance counters")
			}

			return result.counters, nil
		}
	}
}

func performanceSamples(previous map[string]performanceCounters, counters []performanceCounters, now time.Time) ([]metric.Sample, map[string]performanceCounters) {
	next := make(map[string]performanceCounters, len(counters))
	var samples []metric.Sample
	var totalRead, totalWrite float64

	for _, current := range counters {
		number, _, _ := strings.Cut(strings.TrimSpace(current.Name), " ")
		index, err := strconv.ParseUint(number, 10, 32)
		if err != nil || current.Timestamp_Sys100NS == 0 {
			continue
		}
		device := "disk" + strconv.FormatUint(index, 10)
		if _, duplicate := next[device]; duplicate {
			continue
		}
		next[device] = current

		last, seen := previous[device]
		if seen && current.Timestamp_Sys100NS == last.Timestamp_Sys100NS {
			next[device] = last
			continue
		}
		if !seen || current.Timestamp_Sys100NS <= last.Timestamp_Sys100NS ||
			current.DiskReadBytesPerSec < last.DiskReadBytesPerSec ||
			current.DiskWriteBytesPerSec < last.DiskWriteBytesPerSec {
			continue
		}

		ticks := current.Timestamp_Sys100NS - last.Timestamp_Sys100NS
		seconds := float64(ticks) / 1e7
		read := float64(current.DiskReadBytesPerSec-last.DiskReadBytesPerSec) / seconds
		write := float64(current.DiskWriteBytesPerSec-last.DiskWriteBytesPerSec) / seconds
		totalRead += read
		totalWrite += write
		prefix := "diskio." + device + "."
		samples = append(samples,
			metric.Sample{Key: series.Key(prefix + "read"), Value: read, Time: now},
			metric.Sample{Key: series.Key(prefix + "write"), Value: write, Time: now},
		)
		if current.PercentIdleTime >= last.PercentIdleTime && current.PercentIdleTime_Base > last.PercentIdleTime_Base {
			// PERF_PRECISION_100NS_TIMER uses its private base counter, not Timestamp_Sys100NS.
			elapsed := current.PercentIdleTime_Base - last.PercentIdleTime_Base
			idle := float64(current.PercentIdleTime-last.PercentIdleTime) / float64(elapsed)
			samples = append(samples, metric.Sample{Key: series.Key(prefix + "util"), Value: max(0, 100*(1-idle)), Time: now})
		}
	}

	if len(samples) > 0 {
		samples = append(samples,
			metric.Sample{Key: "diskio.total.read", Value: totalRead, Time: now},
			metric.Sample{Key: "diskio.total.write", Value: totalWrite, Time: now},
		)
	}

	return samples, next
}
