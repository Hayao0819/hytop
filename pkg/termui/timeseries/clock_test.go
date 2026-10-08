package timeseries_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func resolutions() []timeseries.Resolution {
	return []timeseries.Resolution{
		{Interval: time.Second, Retention: time.Minute},
		{Interval: 10 * time.Second, Retention: 10 * time.Minute},
	}
}

func TestAStalePointDoesNotLeakIntoTheWindow(t *testing.T) {
	t.Parallel()

	var (
		series = newSeries(t, resolutions())
		origin = time.Now().Truncate(time.Minute)
	)

	for i := range 11 {
		series.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	series.Push(origin.Add(2*time.Second), 999)

	cutoff := origin.Add(5 * time.Second)

	for _, point := range series.Window(origin.Add(10*time.Second), 5*time.Second) {
		if point.Time.Before(cutoff) {
			t.Errorf("a point from %s is outside the window that ends at %s: value %v",
				point.Time.Sub(origin), cutoff.Sub(origin), point.Value)
		}
	}
}

func TestTheHistoryOnlyEverRunsForwards(t *testing.T) {
	t.Parallel()

	var (
		series = newSeries(t, resolutions())
		origin = time.Now().Truncate(time.Minute)
	)

	for i := range 20 {
		series.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	series.Push(origin.Add(3*time.Second), 999)
	series.Push(origin.Add(100*time.Second), 1)

	points := series.Window(origin.Add(200*time.Second), 500*time.Second)

	for i := 1; i < len(points); i++ {
		if !points[i].Time.After(points[i-1].Time) {
			t.Fatalf("point %d is stamped %s, not after %s",
				i, points[i].Time.Sub(origin), points[i-1].Time.Sub(origin))
		}
	}
}

func TestAClockSteppingBackStartsTheHistoryAgain(t *testing.T) {
	t.Parallel()

	var (
		series = newSeries(t, resolutions())
		origin = time.Now().Truncate(time.Hour)
	)

	for i := range 30 {
		series.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	stepped := origin.Add(-time.Hour)

	for i := range 5 {
		series.Push(stepped.Add(time.Duration(i)*time.Second), 42)
	}

	points := series.Window(stepped.Add(5*time.Second), time.Minute)
	if len(points) != 5 {
		t.Fatalf("after the step the history holds %d points, want the 5 that came after it", len(points))
	}

	for _, point := range points {
		if point.Value != 42 {
			t.Errorf("a reading from before the step survived it: %v at %s", point.Value, point.Time)
		}
	}
}

func TestARepeatedTimestampKeepsTheHistory(t *testing.T) {
	t.Parallel()

	var (
		series = newSeries(t, resolutions())
		origin = time.Now().Truncate(time.Minute)
	)

	for i := range 10 {
		series.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	series.Push(origin.Add(9*time.Second), 77)

	if points := series.Window(origin.Add(10*time.Second), time.Minute); len(points) != 10 {
		t.Errorf("the history holds %d points, want the 10 that were pushed", len(points))
	}
}
