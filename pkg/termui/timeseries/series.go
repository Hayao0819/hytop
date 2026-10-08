package timeseries

import (
	"fmt"
	"time"
)

// Resolution bounds a tier to Retention/Interval + 1 observations.
// Interval is the expected sampling interval for the finest tier and the
// averaging bucket width for coarser tiers. Sampling faster than Interval
// shortens the time span retained in the finest tier.
type Resolution struct {
	Interval  time.Duration
	Retention time.Duration
}

// Series stores bounded history at progressively coarser resolutions.
// Storage grows lazily. Construct a Series with NewSeries; callers must
// synchronize concurrent access.
type Series struct {
	tiers []*tier
}

type tier struct {
	resolution Resolution
	ring       *ring

	bucket time.Time
	sum    float64
	count  int
}

// NewSeries creates history with at least one resolution. Intervals must be
// positive, and retention must be at least the interval. Successive tiers must
// increase both values, with each interval a multiple of the preceding one.
// Resolutions are copied, and capacities must fit in an int.
func NewSeries(resolutions []Resolution) (*Series, error) {
	if len(resolutions) == 0 {
		return nil, fmt.Errorf("at least one resolution is required")
	}

	tiers := make([]*tier, len(resolutions))
	for i, resolution := range resolutions {
		if resolution.Interval <= 0 || resolution.Retention < resolution.Interval {
			return nil, fmt.Errorf("resolution %d: require a positive interval and retention >= interval", i)
		}
		if i > 0 {
			previous := resolutions[i-1]
			if resolution.Interval <= previous.Interval || resolution.Retention <= previous.Retention ||
				resolution.Interval%previous.Interval != 0 {
				return nil, fmt.Errorf("resolution %d: require increasing retention and a larger multiple of the previous interval", i)
			}
		}
		capacity := resolution.Retention / resolution.Interval
		if uint64(capacity) >= uint64(^uint(0)>>1) {
			return nil, fmt.Errorf("resolution %d: capacity exceeds int range", i)
		}
		tiers[i] = &tier{resolution: resolution, ring: newRing(int(capacity) + 1)}
	}

	return &Series{tiers: tiers}, nil
}

// Push stores a reading. Repeated timestamps and backward steps smaller than
// the finest interval are ignored; larger backward steps reset history.
// Timestamps must be representable as signed 64-bit Unix nanoseconds.
func (s *Series) Push(t time.Time, v float64) {
	if last, ok := s.tiers[0].ring.last(); ok && !t.After(last.Time) {
		if last.Time.Sub(t) < s.tiers[0].resolution.Interval {
			return
		}

		s.reset()
	}

	s.tiers[0].ring.push(Point{Time: t, Value: v})

	s.feed(1, Point{Time: t, Value: v})
}

func (s *Series) reset() {
	for _, tier := range s.tiers {
		tier.ring.reset()
		tier.bucket, tier.sum, tier.count = time.Time{}, 0, 0
	}
}

func (s *Series) feed(level int, p Point) {
	if level >= len(s.tiers) {
		return
	}

	tier := s.tiers[level]
	bucket := p.Time.Truncate(tier.resolution.Interval)

	if tier.count > 0 && bucket.Before(tier.bucket) {
		return
	}

	if tier.count > 0 && !bucket.Equal(tier.bucket) {
		s.close(level)
	}

	tier.bucket = bucket
	tier.sum += p.Value
	tier.count++
}

func (s *Series) close(level int) {
	tier := s.tiers[level]
	if tier.count == 0 {
		return
	}

	mean := Point{Time: tier.bucket, Value: tier.sum / float64(tier.count)}

	tier.ring.push(mean)
	tier.sum, tier.count = 0, 0

	s.feed(level+1, mean)
}

// Window returns a copy of points since now-span, oldest first, at the finest
// resolution covering span (or the coarsest available). It includes unfinished
// buckets; coarse points are stamped at the start of their bucket.
func (s *Series) Window(now time.Time, span time.Duration) []Point {
	tier := s.tiers[s.pick(span)]

	points := tier.ring.since(now.Add(-span), nil)

	// Include the unfinished coarse bucket to avoid display lag.
	if tier.count > 0 && !tier.bucket.Before(now.Add(-span)) {
		points = append(points, Point{Time: tier.bucket, Value: tier.sum / float64(tier.count)})
	}

	return points
}

func (s *Series) pick(span time.Duration) int {
	for i, tier := range s.tiers {
		if tier.resolution.Retention >= span {
			return i
		}
	}

	return len(s.tiers) - 1
}

// Last returns the most recent raw reading, or false if history is empty.
func (s *Series) Last() (Point, bool) { return s.tiers[0].ring.last() }

// Len returns the number of raw readings retained in the finest tier.
func (s *Series) Len() int { return s.tiers[0].ring.len() }

// Retention is the longest window retained by the series.
func (s *Series) Retention() time.Duration {
	var retention time.Duration
	for _, tier := range s.tiers {
		retention = max(retention, tier.resolution.Retention)
	}

	return retention
}
