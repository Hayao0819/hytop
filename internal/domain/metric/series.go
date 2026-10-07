package metric

import "time"

type Resolution struct {
	Interval  time.Duration
	Retention time.Duration
}

func (r Resolution) capacity() int {
	if r.Interval <= 0 {
		return 1
	}

	return int(r.Retention/r.Interval) + 1
}

func DefaultResolutions() []Resolution {
	return ResolutionsFor(15 * time.Minute)
}

// ResolutionsFor returns progressively coarser tiers covering span.
func ResolutionsFor(span time.Duration) []Resolution {
	span = max(span, time.Second)

	resolutions := make([]Resolution, 0, 3)
	fine := min(span, 10*time.Minute)
	resolutions = append(resolutions, Resolution{Interval: time.Second, Retention: fine})

	if span > fine {
		medium := min(span, 2*time.Hour)
		resolutions = append(resolutions, Resolution{Interval: 10 * time.Second, Retention: medium})
	}

	if span > 2*time.Hour {
		resolutions = append(resolutions, Resolution{Interval: time.Minute, Retention: span})
	}

	return resolutions
}

// Series stores one key's history at several resolutions.
// It is not safe for concurrent use.
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

func NewSeries(resolutions []Resolution) *Series {
	if len(resolutions) == 0 {
		resolutions = DefaultResolutions()
	}

	tiers := make([]*tier, len(resolutions))
	for i, resolution := range resolutions {
		tiers[i] = &tier{resolution: resolution, ring: newRing(resolution.capacity())}
	}

	return &Series{tiers: tiers}
}

// Push stores a reading. A substantial backward clock step resets history to
// preserve chronological ordering; sub-interval repeats are ignored.
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

// Window returns points oldest first at the finest resolution covering span.
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

func (s *Series) Last() (Point, bool) { return s.tiers[0].ring.last() }

func (s *Series) Len() int { return s.tiers[0].ring.len() }

// Retention is the longest window retained by the series.
func (s *Series) Retention() time.Duration {
	var retention time.Duration
	for _, tier := range s.tiers {
		retention = max(retention, tier.resolution.Retention)
	}

	return retention
}
