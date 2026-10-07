package timeseries_test

import (
	"math"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func TestResampleUsesTimeAndHoldsInternalGaps(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	values := timeseries.Resample([]timeseries.Point{
		{Time: now.Add(-8 * time.Second), Value: 2},
		{Time: now.Add(-2 * time.Second), Value: 8},
	}, now, 10*time.Second, 5)

	want := []float64{math.NaN(), 2, 2, 2, 8}
	for i := range want {
		if math.IsNaN(want[i]) != math.IsNaN(values[i]) || (!math.IsNaN(want[i]) && want[i] != values[i]) {
			t.Fatalf("Resample() = %v, want %v", values, want)
		}
	}
}

func TestResampleFuncAcceptsApplicationPointTypes(t *testing.T) {
	t.Parallel()

	type observation struct {
		at    time.Time
		value float64
	}

	now := time.Unix(100, 0)
	values := timeseries.ResampleFunc(
		[]observation{{at: now.Add(-time.Second), value: 7}},
		now,
		2*time.Second,
		2,
		func(point observation) (time.Time, float64) { return point.at, point.value },
	)

	if len(values) != 2 || !math.IsNaN(values[0]) || values[1] != 7 {
		t.Fatalf("ResampleFunc() = %v", values)
	}
}

func TestResampleHandlesLongSpansAndInvalidReadings(t *testing.T) {
	t.Parallel()

	now := time.Unix(1<<32, 0)
	span := 100 * 365 * 24 * time.Hour
	values := timeseries.Resample([]timeseries.Point{
		{Time: now.Add(-span / 2), Value: 7},
		{Time: now.Add(-span / 3), Value: math.Inf(1)},
		{Time: now.Add(-span / 4), Value: math.NaN()},
	}, now, span, 100)

	if len(values) != 100 || values[50] != 7 || values[75] != 7 {
		t.Fatalf("Resample() = %v", values)
	}
}
