package collect

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Hayao0819/hytop/internal/errors"
)

// Job pairs a collector with a dynamically resolved interval.
type Job struct {
	Collector Collector
	Interval  func() time.Duration

	runMu       sync.Mutex
	ready       atomic.Bool
	lastChecked time.Time
}

const (
	collectionTimeout = 10 * time.Second
	availabilityRetry = 30 * time.Second
)

// Scheduler owns collector goroutines and writes their output to a sink.
type Scheduler struct {
	sink Sink
	jobs []*Job

	mu        sync.RWMutex
	problems  map[string]Availability
	factsRead map[string]bool

	timeout           time.Duration
	availabilityRetry time.Duration
}

type SchedulerOption func(*Scheduler)

// WithTimeout supplies each collector facet with a deadline.
func WithTimeout(timeout time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		if timeout > 0 {
			s.timeout = timeout
		}
	}
}

// WithAvailabilityRetry controls how often unavailable collectors are probed.
func WithAvailabilityRetry(interval time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		if interval > 0 {
			s.availabilityRetry = interval
		}
	}
}

func NewScheduler(sink Sink, options ...SchedulerOption) *Scheduler {
	scheduler := &Scheduler{
		sink:              sink,
		problems:          make(map[string]Availability),
		factsRead:         make(map[string]bool),
		timeout:           collectionTimeout,
		availabilityRetry: availabilityRetry,
	}
	for _, option := range options {
		option(scheduler)
	}

	return scheduler
}

func (s *Scheduler) Add(c Collector, interval func() time.Duration) {
	availability := c.Check()
	if !availability.OK() {
		s.mu.Lock()
		s.problems[c.Name()] = availability
		s.mu.Unlock()
	}

	job := &Job{Collector: c, Interval: interval, lastChecked: time.Now()}
	job.ready.Store(availability.OK())
	s.jobs = append(s.jobs, job)
}

// Every is Add for a collector whose period never changes.
func (s *Scheduler) Every(c Collector, interval time.Duration) {
	s.Add(c, func() time.Duration { return interval })
}

func (s *Scheduler) Problems() map[string]Availability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return maps.Clone(s.problems)
}

// Prime performs initial rounds before rendering; rate collectors require two.
func (s *Scheduler) Prime(ctx context.Context, rounds int, gap time.Duration) {
	for round := range max(rounds, 1) {
		if round > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(gap):
			}
		}

		var wg sync.WaitGroup

		for _, job := range s.jobs {
			wg.Go(func() { s.once(ctx, job) })
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			s.markBusy(ctx.Err())

			return
		case <-done:
		}
	}
}

func (s *Scheduler) markBusy(err error) {
	for _, job := range s.jobs {
		if job.runMu.TryLock() {
			job.runMu.Unlock()

			continue
		}

		s.fail(job.Collector.Name(), err)
	}
}

// Run blocks until ctx is done and isolates slow collectors in separate goroutines.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for _, job := range s.jobs {
		wg.Go(func() { s.loop(ctx, job) })
	}

	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, job *Job) {
	interval := s.period(job)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.once(ctx, job)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.once(ctx, job)

			if next := s.period(job); next != interval {
				interval = next

				ticker.Reset(interval)
			}
		}
	}
}

func (s *Scheduler) period(job *Job) time.Duration {
	if !job.ready.Load() {
		return s.availabilityRetry
	}

	if job.Interval == nil {
		return time.Second
	}

	if d := job.Interval(); d > 0 {
		return d
	}

	return time.Second
}

func (s *Scheduler) once(ctx context.Context, job *Job) {
	if !job.runMu.TryLock() {
		return
	}
	defer job.runMu.Unlock()

	if !s.ensureReady(job) {
		return
	}

	runErr := s.collectSamples(ctx, job)
	for _, err := range []error{s.collectFacts(ctx, job), s.collectProcesses(ctx, job)} {
		if runErr == nil {
			runErr = err
		}
	}

	if runErr != nil {
		s.fail(job.Collector.Name(), runErr)

		return
	}

	s.succeed(job.Collector.Name())
}

func (s *Scheduler) ensureReady(job *Job) bool {
	if !job.ready.Load() {
		if time.Since(job.lastChecked) < s.availabilityRetry {
			return false
		}

		availability := job.Collector.Check()
		job.lastChecked = time.Now()
		if !availability.OK() {
			s.unavailable(job.Collector.Name(), availability)

			return false
		}

		job.ready.Store(true)
		s.succeed(job.Collector.Name())
	}

	return true
}

func (s *Scheduler) collectSamples(ctx context.Context, job *Job) error {
	now := time.Now()

	return s.timed(ctx, func(run context.Context) error {
		samples, err := job.Collector.Collect(run, now)
		if len(samples) > 0 {
			s.sink.WriteSamples(samples)
		}

		return err
	})
}

func (s *Scheduler) collectFacts(ctx context.Context, job *Job) error {
	facts, ok := job.Collector.(FactCollector)
	if !ok || !s.shouldReadFacts(job.Collector) {
		return nil
	}

	err := s.timed(ctx, func(run context.Context) error {
		values, err := facts.Facts(run)
		if err == nil {
			s.sink.ReplaceFacts(job.Collector.Name(), values)
		}

		return err
	})
	if err != nil {
		s.releaseFacts(job.Collector.Name())
	}

	return err
}

func (s *Scheduler) collectProcesses(ctx context.Context, job *Job) error {
	processes, ok := job.Collector.(ProcessCollector)
	if !ok {
		return nil
	}

	return s.timed(ctx, func(run context.Context) error {
		values, err := processes.Processes(run)
		if err == nil {
			s.sink.WriteProcesses(values)
		}

		return err
	})
}

func (s *Scheduler) timed(ctx context.Context, run func(context.Context) error) error {
	timed, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	return run(timed)
}

func (s *Scheduler) shouldReadFacts(collector Collector) bool {
	if _, static := collector.(StaticFactCollector); !static {
		return true
	}

	return s.claimFacts(collector.Name())
}

// claimFacts atomically reserves the single read of a static fact collector.
func (s *Scheduler) claimFacts(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.factsRead[name] {
		return false
	}

	s.factsRead[name] = true

	return true
}

func (s *Scheduler) releaseFacts(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.factsRead, name)
}

func (s *Scheduler) fail(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.problems[name] = Availability{
		State:  Failed,
		Reason: errors.Wrapf(err, "collector %s", name).Error(),
	}
}

func (s *Scheduler) unavailable(name string, availability Availability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.problems[name] = availability
}

func (s *Scheduler) succeed(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.problems, name)
}
