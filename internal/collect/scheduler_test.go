package collect_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type recordingSink struct {
	mu      sync.Mutex
	samples []metric.Sample
	facts   int
	sources []string
	procs   int
}

func (s *recordingSink) WriteSamples(samples []metric.Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, samples...)
}

func (s *recordingSink) WriteProcesses([]procmodel.Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.procs++
}

func (s *recordingSink) ReplaceFacts(source string, _ map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts++
	s.sources = append(s.sources, source)
}

type scriptedCollector struct {
	collect func(context.Context, time.Time) ([]metric.Sample, error)
}

func (scriptedCollector) Name() string                { return "scripted" }
func (scriptedCollector) Check() collect.Availability { return collect.Availability{} }
func (c scriptedCollector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	return c.collect(ctx, now)
}

func TestSchedulerKeepsPartialReadingsAndClearsRecoveredProblems(t *testing.T) {
	t.Parallel()

	sink := &recordingSink{}
	calls := 0
	collector := scriptedCollector{collect: func(_ context.Context, now time.Time) ([]metric.Sample, error) {
		calls++
		samples := []metric.Sample{{Key: "test.value", Value: float64(calls), Time: now}}
		if calls == 1 {
			return samples, errors.New("one backend failed")
		}
		return samples, nil
	}}
	scheduler := collect.NewScheduler(sink)
	scheduler.Every(collector, time.Second)

	scheduler.Prime(t.Context(), 1, 0)
	if len(sink.samples) != 1 || len(scheduler.Problems()) != 1 {
		t.Fatalf("partial run: samples=%v problems=%v", sink.samples, scheduler.Problems())
	}

	scheduler.Prime(t.Context(), 1, 0)
	if len(sink.samples) != 2 || len(scheduler.Problems()) != 0 {
		t.Fatalf("recovered run: samples=%v problems=%v", sink.samples, scheduler.Problems())
	}
}

func TestSchedulerPassesCancellationIntoACollector(t *testing.T) {
	t.Parallel()

	scheduler := collect.NewScheduler(&recordingSink{}, collect.WithTimeout(20*time.Millisecond))
	scheduler.Every(scriptedCollector{collect: func(ctx context.Context, _ time.Time) ([]metric.Sample, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}, time.Second)

	started := time.Now()
	scheduler.Prime(t.Context(), 1, 0)

	if time.Since(started) > time.Second {
		t.Fatal("collector ignored the scheduler context")
	}
	if len(scheduler.Problems()) != 1 {
		t.Fatalf("cancelled collector problem was not recorded: %v", scheduler.Problems())
	}
}

func TestPrimeReturnsWhenACollectorIgnoresCancellation(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	defer close(blocked)
	scheduler := collect.NewScheduler(&recordingSink{})
	scheduler.Every(scriptedCollector{collect: func(context.Context, time.Time) ([]metric.Sample, error) {
		<-blocked

		return nil, nil
	}}, time.Second)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	started := time.Now()
	scheduler.Prime(ctx, 1, 0)

	if time.Since(started) > time.Second {
		t.Fatal("a non-cooperative collector held startup open")
	}
	if len(scheduler.Problems()) != 1 {
		t.Fatalf("wedged collector was not reported: %v", scheduler.Problems())
	}
}

type recoveringCollector struct {
	ready atomic.Bool
	calls atomic.Int32
}

func (*recoveringCollector) Name() string { return "recovering" }
func (c *recoveringCollector) Check() collect.Availability {
	if c.ready.Load() {
		return collect.Availability{State: collect.Ready}
	}

	return collect.Availability{State: collect.NoHardware, Reason: "not yet"}
}

func (c *recoveringCollector) Collect(context.Context, time.Time) ([]metric.Sample, error) {
	c.calls.Add(1)

	return nil, nil
}

func TestSchedulerRetriesAnUnavailableCollector(t *testing.T) {
	t.Parallel()

	collector := &recoveringCollector{}
	scheduler := collect.NewScheduler(
		&recordingSink{}, collect.WithAvailabilityRetry(5*time.Millisecond),
	)
	scheduler.Every(collector, time.Hour)
	if len(scheduler.Problems()) != 1 {
		t.Fatal("initial availability problem was not recorded")
	}

	collector.ready.Store(true)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for collector.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done

	if collector.calls.Load() == 0 || len(scheduler.Problems()) != 0 {
		t.Fatalf("calls=%d problems=%v", collector.calls.Load(), scheduler.Problems())
	}
}

type factCollector struct {
	scriptedCollector
	facts int
}

func (c *factCollector) Facts(context.Context) (collect.Facts, error) {
	c.facts++

	return collect.Facts{"value": "ready"}, nil
}

type staticFactCollector struct{ factCollector }

func (*staticFactCollector) StaticFacts() {}

func TestSchedulerRefreshesFactsUnlessTheyAreStatic(t *testing.T) {
	t.Parallel()

	read := func(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
	dynamic := &factCollector{scriptedCollector: scriptedCollector{collect: read}}
	static := &staticFactCollector{factCollector{scriptedCollector: scriptedCollector{collect: read}}}

	for _, collector := range []collect.Collector{dynamic, static} {
		scheduler := collect.NewScheduler(&recordingSink{})
		scheduler.Every(collector, time.Second)
		scheduler.Prime(t.Context(), 2, 0)
	}

	if dynamic.facts != 2 || static.facts != 1 {
		t.Fatalf("dynamic facts=%d static facts=%d", dynamic.facts, static.facts)
	}
}

type slowFacets struct{ scriptedCollector }

func (slowFacets) Facts(ctx context.Context) (collect.Facts, error) {
	select {
	case <-time.After(15 * time.Millisecond):
		return collect.Facts{"ready": "yes"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (slowFacets) Processes(ctx context.Context) ([]procmodel.Process, error) {
	select {
	case <-time.After(15 * time.Millisecond):
		return []procmodel.Process{{PID: 1}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSchedulerGivesEachFacetItsOwnTimeout(t *testing.T) {
	t.Parallel()

	sink := &recordingSink{}
	collector := slowFacets{scriptedCollector{collect: func(ctx context.Context, _ time.Time) ([]metric.Sample, error) {
		select {
		case <-time.After(15 * time.Millisecond):
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}}
	scheduler := collect.NewScheduler(sink, collect.WithTimeout(25*time.Millisecond))
	scheduler.Every(collector, time.Second)
	scheduler.Prime(t.Context(), 1, 0)

	if sink.facts != 1 || sink.procs != 1 || len(sink.sources) != 1 || sink.sources[0] != "scripted" ||
		len(scheduler.Problems()) != 0 {
		t.Fatalf("facts=%d sources=%v processes=%d problems=%v",
			sink.facts, sink.sources, sink.procs, scheduler.Problems())
	}
}
