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
