package timeseries_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

var origin = time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

func newSeries(t *testing.T, resolutions []timeseries.Resolution) *timeseries.Series {
	t.Helper()

	history, err := timeseries.NewSeries(resolutions)
	if err != nil {
		t.Fatal(err)
	}

	return history
}

func TestRingDropsTheOldestWhenFull(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{{Interval: time.Second, Retention: 2 * time.Second}})

	for i := range 10 {
		s.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	if got := s.Len(); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}

	last, ok := s.Last()
	if !ok || last.Value != 9 {
		t.Fatalf("Last = %v, %v, want value 9", last, ok)
	}

	points := s.Window(origin.Add(9*time.Second), time.Hour)
	if len(points) != 3 || points[0].Value != 7 || points[2].Value != 9 {
		t.Fatalf("Window = %v, want the last three", points)
	}
}

func TestRingKeepsItsLengthAtTheFirstWrap(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{{Interval: time.Second, Retention: 2 * time.Second}})
	for i := range 4 {
		s.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	points := s.Window(origin.Add(3*time.Second), time.Hour)
	if s.Len() != 3 || len(points) != 3 || points[0].Value != 1 || points[2].Value != 3 {
		t.Fatalf("first wrapped window = %v (len %d), want [1 2 3]", points, s.Len())
	}
}

func TestCoarseTiersAverageTheFineOnes(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{
		{Interval: time.Second, Retention: time.Minute},
		{Interval: 10 * time.Second, Retention: time.Hour},
	})

	for i := range 20 {
		s.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	now := origin.Add(20 * time.Second)

	points := s.Window(now, 30*time.Minute)
	if len(points) != 2 {
		t.Fatalf("Window = %v, want two ten-second means", points)
	}

	if points[0].Value != 4.5 {
		t.Errorf("first bucket = %v, want mean of 0..9", points[0].Value)
	}

	if points[1].Value != 14.5 {
		t.Errorf("second bucket = %v, want mean of 10..19", points[1].Value)
	}
}

func TestWindowShowsTheOpenBucket(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{
		{Interval: time.Second, Retention: time.Minute},
		{Interval: 10 * time.Second, Retention: time.Hour},
	})

	for i := range 5 {
		s.Push(origin.Add(time.Duration(i)*time.Second), 100)
	}

	points := s.Window(origin.Add(5*time.Second), time.Hour)
	if len(points) != 1 || points[0].Value != 100 {
		t.Fatalf("Window = %v, want the open bucket", points)
	}
}

func TestWindowPicksTheFinestTierThatReachesBack(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{
		{Interval: time.Second, Retention: 10 * time.Second},
		{Interval: 10 * time.Second, Retention: time.Hour},
	})

	for i := range 30 {
		s.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	now := origin.Add(30 * time.Second)

	fine := s.Window(now, 5*time.Second)
	if len(fine) != 5 {
		t.Errorf("five-second window = %d points, want 5 from the fine tier", len(fine))
	}

	coarse := s.Window(now, 20*time.Minute)
	if len(coarse) > 4 {
		t.Errorf("twenty-minute window = %d points, want the coarse tier's few", len(coarse))
	}
}

func TestWindowExcludesWhatFellOutOfIt(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{{Interval: time.Second, Retention: time.Hour}})

	for i := range 60 {
		s.Push(origin.Add(time.Duration(i)*time.Second), float64(i))
	}

	points := s.Window(origin.Add(60*time.Second), 10*time.Second)
	if len(points) != 10 || points[0].Value != 50 {
		t.Fatalf("Window = %d points starting at %v, want 10 starting at 50", len(points), points[0].Value)
	}
}

func TestEmptySeries(t *testing.T) {
	t.Parallel()

	s := newSeries(t, []timeseries.Resolution{{Interval: time.Second, Retention: time.Minute}})

	if _, ok := s.Last(); ok {
		t.Error("Last on an empty series reported a point")
	}

	if got := s.Window(origin, time.Minute); len(got) != 0 {
		t.Errorf("Window on an empty series = %v", got)
	}
}

func TestWindowBeyondCompactTimestampBounds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		at   time.Time
		now  time.Time
		span time.Duration
		want int
	}{
		{"before minimum", time.Unix(0, -1<<63), time.Unix(0, -1<<63), time.Second, 1},
		{"after maximum", time.Unix(0, 1<<63-1), time.Unix(0, 1<<63-1).Add(2 * time.Second), time.Second, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newSeries(t, []timeseries.Resolution{{Interval: time.Second, Retention: time.Minute}})
			s.Push(tc.at, 42)
			if points := s.Window(tc.now, tc.span); len(points) != tc.want {
				t.Fatalf("Window at %s = %v, want %d points", tc.now, points, tc.want)
			}
		})
	}
}
