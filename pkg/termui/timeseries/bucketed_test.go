package timeseries_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func TestBucketedSeriesAveragesHistoryAndKeepsTheLatestReading(t *testing.T) {
	t.Parallel()

	s, err := timeseries.NewBucketedSeries([]timeseries.Resolution{
		{Interval: time.Second, Retention: 3 * time.Second},
		{Interval: 10 * time.Second, Retention: time.Minute},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Last(); ok || s.Len() != 0 {
		t.Fatal("new bucketed history is not empty")
	}
	s.Push(origin.Add(100*time.Millisecond), 10)
	s.Push(origin.Add(900*time.Millisecond), 30)
	points := s.Window(origin.Add(900*time.Millisecond), 2*time.Second)
	if len(points) != 1 || points[0].Value != 20 || !points[0].Time.Equal(origin.Add(900*time.Millisecond)) {
		t.Fatalf("unfinished fine bucket = %v, want mean 20 at the latest reading", points)
	}
	last, ok := s.Last()
	if !ok || last.Value != 30 || !last.Time.Equal(origin.Add(900*time.Millisecond)) {
		t.Fatalf("latest reading = %v, %v", last, ok)
	}
	s.Push(origin.Add(time.Second), 50)
	points = s.Window(origin.Add(time.Second), 2*time.Second)
	if len(points) != 2 || points[0].Value != 20 || points[1].Value != 50 || s.Len() != 2 {
		t.Fatalf("fine buckets = %v, len=%d", points, s.Len())
	}
	coarse := s.Window(origin.Add(time.Second), time.Minute)
	if len(coarse) != 1 || coarse[0].Value != 30 {
		t.Fatalf("coarse bucket = %v, want mean of all three raw readings", coarse)
	}
}

func TestBucketedHistoryKeepsLiveSubsecondWindows(t *testing.T) {
	t.Parallel()

	s, err := timeseries.NewBucketedSeries(resolutions())
	if err != nil {
		t.Fatal(err)
	}
	s.Push(origin.Add(100*time.Millisecond), 10)
	s.Push(origin.Add(900*time.Millisecond), 30)
	points := s.Window(origin.Add(950*time.Millisecond), 100*time.Millisecond)
	if len(points) != 1 || points[0].Value != 20 || !points[0].Time.Equal(origin.Add(900*time.Millisecond)) {
		t.Fatalf("live 100ms window = %v", points)
	}
}

func TestBucketStartsStayWithinCompactTimestampBounds(t *testing.T) {
	t.Parallel()

	for _, newHistory := range []func([]timeseries.Resolution) (*timeseries.Series, error){
		timeseries.NewSeries, timeseries.NewBucketedSeries,
	} {
		s, err := newHistory(resolutions())
		if err != nil {
			t.Fatal(err)
		}
		first := time.Unix(0, -1<<63)
		for i := range 12 {
			s.Push(first.Add(time.Duration(i)*time.Second), float64(i))
		}
		now := first.Add(12 * time.Second)
		for _, span := range []time.Duration{time.Minute, 5 * time.Minute} {
			points := s.Window(now, span)
			if len(points) == 0 {
				t.Fatal("history at the compact timestamp boundary is empty")
			}
			for i, point := range points {
				if point.Time.Before(first) || point.Time.After(now) || i > 0 && !point.Time.After(points[i-1].Time) {
					t.Fatalf("invalid or unordered compact bucket: %v", points)
				}
			}
		}
	}
}

func TestBucketedHistoryIgnoresDuplicatesAndResetsAfterAClockStep(t *testing.T) {
	t.Parallel()

	s, err := timeseries.NewBucketedSeries(resolutions())
	if err != nil {
		t.Fatal(err)
	}
	s.Push(origin.Add(1100*time.Millisecond), 10)
	s.Push(origin.Add(1200*time.Millisecond), 20)
	s.Push(origin.Add(1200*time.Millisecond), 999)
	s.Push(origin.Add(time.Second), 888)
	if last, ok := s.Last(); !ok || last.Value != 20 || s.Len() != 1 {
		t.Fatalf("duplicate or short backward step changed history: %v, %v len=%d", last, ok, s.Len())
	}
	s.Push(origin.Add(-5*time.Second), 42)
	for _, span := range []time.Duration{time.Second, 5 * time.Minute} {
		points := s.Window(origin.Add(-5*time.Second), span)
		if len(points) != 1 || points[0].Value != 42 {
			t.Fatalf("%s window after clock step = %v, want only 42", span, points)
		}
	}
}

func TestBucketedConstructorValidatesResolutions(t *testing.T) {
	t.Parallel()

	for _, resolutions := range [][]timeseries.Resolution{
		nil,
		{{Interval: 0, Retention: time.Second}},
		{{Interval: time.Second, Retention: 0}},
	} {
		if _, err := timeseries.NewBucketedSeries(resolutions); err == nil {
			t.Fatalf("invalid bucketed resolutions were accepted: %v", resolutions)
		}
	}
}
