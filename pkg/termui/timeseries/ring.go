package timeseries

import "time"

// compactPoint avoids storing the 24-byte time.Time in each history slot.
type compactPoint struct {
	unixNano int64
	value    float64
}

type ring struct {
	points []compactPoint
	limit  int
	next   int
}

const initialRing = 8

func newRing(limit int) *ring {
	if limit < 1 {
		limit = 1
	}

	return &ring{limit: limit}
}

func (r *ring) push(p Point) {
	compact := compactPoint{unixNano: p.Time.UnixNano(), value: p.Value}

	if len(r.points) < r.limit {
		if len(r.points) == cap(r.points) {
			r.grow()
		}
		r.points = append(r.points, compact)
		if len(r.points) == r.limit {
			r.next = 0
		}

		return
	}

	r.points[r.next] = compact
	r.next = (r.next + 1) % len(r.points)
}

func (r *ring) grow() {
	capacity := initialRing
	if current := cap(r.points); current > 0 {
		capacity = current + min(current, r.limit-current)
	}
	capacity = min(capacity, r.limit)

	points := make([]compactPoint, len(r.points), capacity)
	copy(points, r.points)
	r.points = points
}

func (r *ring) len() int { return len(r.points) }

func (r *ring) rawAt(i int) compactPoint {
	if len(r.points) < r.limit {
		return r.points[i]
	}

	return r.points[(r.next+i)%len(r.points)]
}

func (r *ring) at(i int) Point {
	p := r.rawAt(i)

	return Point{Time: time.Unix(0, p.unixNano), Value: p.value}
}

func (r *ring) since(t time.Time, dst []Point) []Point {
	n := r.len()
	cutoff := t.UnixNano()
	// Query bounds can extend beyond the compact timestamp range.
	if restored := time.Unix(0, cutoff); !restored.Equal(t) {
		if restored.Before(t) {
			return dst
		}
		cutoff = -1 << 63
	}

	start := 0
	for start < n && r.rawAt(start).unixNano < cutoff {
		start++
	}

	for i := start; i < n; i++ {
		dst = append(dst, r.at(i))
	}

	return dst
}

func (r *ring) reset() {
	r.points = r.points[:0]
	r.next = 0
}

func (r *ring) last() (Point, bool) {
	if r.len() == 0 {
		return Point{}, false
	}

	return r.at(r.len() - 1), true
}
