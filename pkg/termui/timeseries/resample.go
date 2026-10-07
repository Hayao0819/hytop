// Package timeseries transforms timestamped numeric observations.
package timeseries

import (
	"math"
	"time"
)

// Point is one timestamped numeric observation.
type Point struct {
	Time  time.Time
	Value float64
}

// Resample spreads timestamped readings over a fixed number of plot slots.
// Multiple readings in one slot are averaged and a reading is held through an
// internal gap. The area before the first reading remains empty.
func Resample(points []Point, now time.Time, span time.Duration, slots int) []float64 {
	return ResampleFunc(points, now, span, slots, func(point Point) (time.Time, float64) {
		return point.Time, point.Value
	})
}

// ResampleFunc applies Resample semantics to any point type using observe.
func ResampleFunc[T any](
	points []T,
	now time.Time,
	span time.Duration,
	slots int,
	observe func(T) (time.Time, float64),
) []float64 {
	out := make([]float64, max(0, slots))
	for i := range out {
		out[i] = math.NaN()
	}

	if slots <= 0 || len(points) == 0 || span <= 0 {
		return out
	}

	start := now.Add(-span)
	sums := make([]float64, slots)
	counts := make([]int, slots)

	for _, point := range points {
		at, value := observe(point)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		elapsed := at.Sub(start)
		if elapsed < 0 || elapsed > span {
			continue
		}

		slot := int(float64(elapsed) / float64(span) * float64(slots))
		slot = min(slot, slots-1)
		if slot >= 0 && slot < slots {
			sums[slot] += value
			counts[slot]++
		}
	}

	last := math.NaN()
	for i := range out {
		if counts[i] > 0 {
			out[i] = sums[i] / float64(counts[i])
			last = out[i]
		} else if !math.IsNaN(last) {
			out[i] = last
		}
	}

	return out
}
