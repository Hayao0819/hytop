package metric_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
)

func TestResolutionsOnlyCoverTheRequestedSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		span  time.Duration
		tiers int
		last  time.Duration
	}{
		{span: time.Minute, tiers: 1, last: time.Minute},
		{span: 15 * time.Minute, tiers: 2, last: 15 * time.Minute},
		{span: 3 * time.Hour, tiers: 3, last: 3 * time.Hour},
	}

	for _, test := range tests {
		got := metric.ResolutionsFor(test.span)
		if len(got) != test.tiers {
			t.Fatalf("ResolutionsFor(%s) returned %d tiers, want %d", test.span, len(got), test.tiers)
		}
		if got[len(got)-1].Retention != test.last {
			t.Fatalf("ResolutionsFor(%s) retains %s, want %s", test.span, got[len(got)-1].Retention, test.last)
		}
	}
}

func TestNewSeriesKeepsHytopDefaults(t *testing.T) {
	t.Parallel()

	for _, resolutions := range [][]metric.Resolution{
		nil,
		metric.DefaultResolutions(),
	} {
		history := metric.NewSeries(resolutions)
		if got := history.Retention(); got != 15*time.Minute {
			t.Fatalf("default retention = %s, want 15m", got)
		}
		if _, ok := history.Last(); ok {
			t.Fatal("new history contains a reading")
		}
		now := time.Now()
		history.Push(now, 42)
		if points := history.Window(now, time.Minute); len(points) != 1 || points[0].Value != 42 {
			t.Fatalf("default history window = %v, want a single reading of 42", points)
		}
	}
}
