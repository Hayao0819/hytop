package metric

import (
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

type Resolution = timeseries.Resolution

type Series = timeseries.Series

func DefaultResolutions() []Resolution {
	return ResolutionsFor(15 * time.Minute)
}

// ResolutionsFor returns progressively coarser tiers covering span.
func ResolutionsFor(span time.Duration) []Resolution {
	span = max(span, time.Second)

	resolutions := make([]Resolution, 0, 3)
	fine := min(span, 10*time.Minute)
	resolutions = append(resolutions, Resolution{Interval: time.Second, Retention: fine})

	if span > fine {
		medium := min(span, 2*time.Hour)
		resolutions = append(resolutions, Resolution{Interval: 10 * time.Second, Retention: medium})
	}

	if span > 2*time.Hour {
		resolutions = append(resolutions, Resolution{Interval: time.Minute, Retention: span})
	}

	return resolutions
}

// NewSeries applies hytop defaults and panics for invalid history policies.
func NewSeries(resolutions []Resolution) *Series {
	if len(resolutions) == 0 {
		resolutions = DefaultResolutions()
	}

	history, err := timeseries.NewSeries(resolutions)
	if err != nil {
		panic(err)
	}

	return history
}
