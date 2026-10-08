package widget

import (
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func Resample(points []metric.Point, now time.Time, span time.Duration, slots int) []float64 {
	return timeseries.Resample(points, now, span, slots)
}
