package timeseries

import (
	"fmt"
	"time"
)

// Resolution bounds a tier to Retention/Interval + 1 points.
// Interval is NewSeries' expected sampling interval for its finest tier;
// faster sampling shortens that tier's retained span. Other tiers and
// NewBucketedSeries use Interval as their averaging bucket width.
type Resolution struct {
	Interval  time.Duration
	Retention time.Duration
}

// Series stores bounded history at progressively coarser resolutions.
// Storage grows lazily. Construct with NewSeries or NewBucketedSeries;
// callers must synchronize concurrent access.
type Series struct {
	tiers    []*tier
	bucketed bool
	latest   Point
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

// NewBucketedSeries averages readings within each finest-tier interval.
// Last returns the latest raw reading; resolution validation follows NewSeries.
func NewBucketedSeries(resolutions []Resolution) (*Series, error) {
	s, err := NewSeries(resolutions)
	if err != nil {
		return nil, err
	}
	s.bucketed = true
	s.tiers[0].ring.limit--

	return s, nil
}

// Push stores a reading. Repeated timestamps and backward steps smaller than
// the finest interval are ignored; larger backward steps reset history.
// Timestamps must be representable as signed 64-bit Unix nanoseconds.
func (s *Series) Push(t time.Time, v float64) {
	if last, ok := s.Last(); ok && !t.After(last.Time) {
		if last.Time.Sub(t) < s.tiers[0].resolution.Interval {
			return
		}

		s.reset()
	}

	p := Point{Time: t, Value: v}
	if s.bucketed {
		s.latest = Point{Time: t.Round(0), Value: v}
		s.feed(0, p)
	} else {
		s.tiers[0].ring.push(p)
	}
	s.feed(1, p)
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
	if earliest := time.Unix(0, -1<<63); bucket.Before(earliest) {
		bucket = earliest
	}

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

	if level > 0 {
		s.feed(level+1, mean)
	}
}

// Window returns a copy of points since now-span, oldest first, at the finest
// resolution covering span (or the coarsest available). It includes unfinished
// buckets. An unfinished finest bucket uses the latest reading's timestamp;
// other buckets use their start.
func (s *Series) Window(now time.Time, span time.Duration) []Point {
	tier := s.tiers[s.pick(span)]

	points := tier.ring.since(now.Add(-span), nil)

	// Include the unfinished coarse bucket to avoid display lag.
	if tier.count > 0 {
		stamp := tier.bucket
		if s.bucketed && tier == s.tiers[0] {
			stamp = s.latest.Time
		}
		if !stamp.Before(now.Add(-span)) {
			points = append(points, Point{Time: stamp, Value: tier.sum / float64(tier.count)})
		}
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
func (s *Series) Last() (Point, bool) {
	if s.bucketed && s.tiers[0].count > 0 {
		return s.latest, true
	}

	return s.tiers[0].ring.last()
}

// Len returns the number of finest-tier points, including an unfinished bucket.
func (s *Series) Len() int {
	n := s.tiers[0].ring.len()
	if s.tiers[0].count > 0 {
		n++
	}

	return n
}

// Retention is the longest window retained by the series.
func (s *Series) Retention() time.Duration {
	var retention time.Duration
	for _, tier := range s.tiers {
		retention = max(retention, tier.resolution.Retention)
	}

	return retention
}
