package widget

import (
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

// Resample adapts hytop's metric points to the public time-series primitive.
func Resample(points []metric.Point, now time.Time, span time.Duration, slots int) []float64 {
	return timeseries.ResampleFunc(points, now, span, slots, func(point metric.Point) (time.Time, float64) {
		return point.Time, point.Value
	})
}
