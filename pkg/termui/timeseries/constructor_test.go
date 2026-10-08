package timeseries_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func TestNewSeriesRejectsInvalidResolutions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		resolutions []timeseries.Resolution
	}{
		{"empty", nil},
		{"zero interval", []timeseries.Resolution{{Retention: time.Second}}},
		{"negative interval", []timeseries.Resolution{{Interval: -time.Second, Retention: time.Second}}},
		{"negative retention", []timeseries.Resolution{{Interval: time.Second, Retention: -time.Second}}},
		{"short retention", []timeseries.Resolution{{Interval: time.Second, Retention: time.Millisecond}}},
		{"duplicate interval", []timeseries.Resolution{
			{Interval: time.Second, Retention: time.Minute},
			{Interval: time.Second, Retention: time.Hour},
		}},
		{"decreasing interval", []timeseries.Resolution{
			{Interval: 10 * time.Second, Retention: time.Minute},
			{Interval: time.Second, Retention: time.Hour},
		}},
		{"duplicate retention", []timeseries.Resolution{
			{Interval: time.Second, Retention: time.Minute},
			{Interval: 10 * time.Second, Retention: time.Minute},
		}},
		{"decreasing retention", []timeseries.Resolution{
			{Interval: time.Second, Retention: time.Hour},
			{Interval: 10 * time.Second, Retention: time.Minute},
		}},
		{"unaligned intervals", []timeseries.Resolution{
			{Interval: 3 * time.Second, Retention: time.Minute},
			{Interval: 10 * time.Second, Retention: time.Hour},
		}},
		{"capacity overflow", []timeseries.Resolution{{Interval: time.Nanosecond, Retention: 1<<63 - 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if s, err := timeseries.NewSeries(tc.resolutions); err == nil || s != nil {
				t.Fatalf("NewSeries(%v) = %v, %v; want nil and error", tc.resolutions, s, err)
			}
		})
	}
}

func TestSeriesOwnsItsResolutionsAndWindows(t *testing.T) {
	t.Parallel()

	resolutions := []timeseries.Resolution{{Interval: time.Second, Retention: time.Minute}}
	s := newSeries(t, resolutions)
	resolutions[0].Retention = time.Hour
	if got := s.Retention(); got != time.Minute {
		t.Fatalf("Retention after caller mutation = %s, want 1m", got)
	}

	s.Push(origin, 42)
	points := s.Window(origin, time.Minute)
	points[0].Value = 99
	if last, _ := s.Last(); last.Value != 42 {
		t.Fatalf("Last after window mutation = %v, want 42", last)
	}
}

func TestNewSeriesAllowsLargeLazyCapacity(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{
		{Interval: time.Nanosecond, Retention: time.Duration(^uint(0)>>1) - 1},
	})
	s.Push(origin, 42)
	if s.Len() != 1 {
		t.Fatalf("large-capacity series has %d points, want 1", s.Len())
	}
}

func ExampleNewSeries() {
	history, err := timeseries.NewSeries([]timeseries.Resolution{
		{Interval: time.Second, Retention: time.Minute},
		{Interval: 10 * time.Second, Retention: time.Hour},
	})
	if err != nil {
		panic(err)
	}
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	history.Push(now, 42)
	fmt.Println(history.Window(now, time.Minute)[0].Value)
	// Output: 42
}
