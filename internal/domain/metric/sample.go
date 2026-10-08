// Package metric stores keyed measurements and hytop's history policies.
package metric

import (
	"time"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

type Sample struct {
	Key   series.Key
	Value float64
	Time  time.Time
}

type Point = timeseries.Point
