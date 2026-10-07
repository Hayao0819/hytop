package metric

import (
	"testing"
	"time"
)

func TestRingAllocatesLazilyAndStopsAtItsLimit(t *testing.T) {
	t.Parallel()

	ring := newRing(10)
	if capacity := cap(ring.points); capacity != 0 {
		t.Fatalf("new ring capacity = %d, want 0", capacity)
	}

	for i := range 9 {
		ring.push(Point{Time: time.Unix(int64(i), 0), Value: float64(i)})
	}
	if capacity := cap(ring.points); capacity != 10 {
		t.Fatalf("grown ring capacity = %d, want limit 10", capacity)
	}

	for i := 9; i < 20; i++ {
		ring.push(Point{Time: time.Unix(int64(i), 0), Value: float64(i)})
	}
	if len(ring.points) != 10 || cap(ring.points) != 10 {
		t.Fatalf("full ring = len %d cap %d, want 10/10", len(ring.points), cap(ring.points))
	}
}
