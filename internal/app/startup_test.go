package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"
	"github.com/charmbracelet/x/ansi"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

func startupEnv() Env {
	memory := store.New(metric.DefaultResolutions())
	config := conf.Default()
	config.Appearance.Background = conf.BackgroundDark
	return Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Scheduler: collect.NewScheduler(memory), Config: config,
		Caps: render.Caps{Glyphs: render.Braille, Colors: render.Ansi256},
	}
}

type startupOutput struct {
	mu      sync.Mutex
	text    strings.Builder
	changed chan struct{}
}

func (o *startupOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	n, err := o.text.Write(p)
	o.mu.Unlock()
	select {
	case o.changed <- struct{}{}:
	default:
	}
	return n, err
}

func (o *startupOutput) wait(t *testing.T, text string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		o.mu.Lock()
		got := ansi.Strip(o.text.String())
		o.mu.Unlock()
		if strings.Contains(got, text) {
			return
		}
		select {
		case <-o.changed:
		case <-deadline.C:
			t.Fatalf("UI did not render %q during initialization:\n%s", text, got)
		}
	}
}

func TestStartupRendersAndHandlesInputBeforeInitializationFinishes(t *testing.T) {
	t.Parallel()
	env := startupEnv()
	started, stopped := make(chan struct{}), make(chan struct{})
	env.Initialize = func(ctx context.Context) (Sources, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return Sources{}, ctx.Err()
	}
	output := &startupOutput{changed: make(chan struct{}, 1)}
	app := reactea.New(modal.New(New(env)), reactea.WithRoute("/graphs/cpu"))
	program := app.Program(tea.WithInput(nil), tea.WithOutput(output),
		tea.WithWindowSize(100, 28), tea.WithoutSignalHandler())
	t.Cleanup(program.Kill)
	t.Cleanup(app.Scope().Close)
	done := make(chan error, 1)
	go func() {
		_, err := program.Run()
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("background initialization did not start")
	}
	output.wait(t, "starting…")
	program.Send(tea.KeyPressMsg{Code: '0'})
	output.wait(t, "Graph history")
	program.Send(tea.KeyPressMsg{Code: 'q'})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quit waited for background initialization")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("quit did not cancel background initialization")
	}
}

func TestStartupKeepsTheRequestedGraphAndPublishesSourcesWithoutRemounting(t *testing.T) {
	t.Parallel()
	env := startupEnv()
	first, finish := make(chan struct{}), make(chan struct{})
	env.Initialize = func(ctx context.Context) (Sources, error) {
		env.Store.WriteSamples([]metric.Sample{{Key: "mem.usage", Value: 42, Time: time.Now()}})
		close(first)
		select {
		case <-finish:
		case <-ctx.Done():
			return Sources{}, ctx.Err()
		}
		env.Store.WriteSamples([]metric.Sample{{Key: "cpu.total.usage", Value: 25, Time: time.Now()}})
		return Sources{GPUs: func() []int { return []int{7} }}, nil
	}
	root := New(env)
	app := reactea.New(modal.New(root), reactea.WithRoute("/graphs/cpu"), reactea.WithSize(100, 28))
	t.Cleanup(app.Scope().Close)
	_ = app.Init()
	command := root.initialize(app.Ctx().Context())
	done := make(chan tea.Msg, 1)
	go func() { done <- command() }()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("first readings did not arrive")
	}
	app.Send(tea.WindowSizeMsg{Width: 100, Height: 28})
	if route := app.Route(); route != "/graphs/cpu" {
		t.Fatalf("partial readings changed the requested page to %q", route)
	}
	if view := ansi.Strip(app.View().Content); !strings.Contains(view, "starting…") || !strings.Contains(view, "Memory") {
		t.Fatalf("first readings were not rendered while starting:\n%s", view)
	}
	processes := root.processes
	for _, key := range []rune{'2', '/', 'v'} {
		app.Send(tea.KeyPressMsg{Code: key, Text: string(key)})
	}
	if !app.InputCaptured() {
		t.Fatal("filter input did not open during initialization")
	}
	close(finish)
	select {
	case msg := <-done:
		app.Send(msg)
	case <-time.After(5 * time.Second):
		t.Fatal("initialization did not finish")
	}
	if root.processes != processes || !app.InputCaptured() {
		t.Fatal("initialization remounted the page or discarded in-progress input")
	}
	if root.starting || root.startErr != nil {
		t.Fatalf("startup state = %v, %v", root.starting, root.startErr)
	}
	app.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	app.Send(tea.KeyPressMsg{Code: '1'})
	if view := ansi.Strip(app.View().Content); !strings.Contains(view, "GPU 7") || strings.Contains(view, "starting…") {
		t.Fatalf("completed discovery did not update existing pages:\n%s", view)
	}
}

func TestRunReturnsBackgroundInitializationErrors(t *testing.T) {
	t.Parallel()
	env := startupEnv()
	failure := errors.New("cannot open metrics source")
	env.Initialize = func(context.Context) (Sources, error) { return Sources{}, failure }
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := run(ctx, env, tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutRenderer(), tea.WithoutSignalHandler(), tea.WithWindowSize(100, 28))
	if !errors.Is(err, failure) {
		t.Fatalf("startup error = %v, want %v", err, failure)
	}
}

type startupCollector struct {
	calls   atomic.Int32
	running chan struct{}
	stopped chan struct{}
}

func (*startupCollector) Name() string { return "startup" }
func (*startupCollector) Check() collect.Availability {
	return collect.Availability{State: collect.Ready}
}

func (c *startupCollector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	if c.calls.Add(1) <= 2 {
		return []metric.Sample{{Key: "cpu.total.usage", Value: 25, Time: now}}, nil
	}
	close(c.running)
	<-ctx.Done()
	close(c.stopped)
	return nil, ctx.Err()
}

func TestStartupHandsPrimedCollectorsToTheSchedulerAndStopsWithItsScope(t *testing.T) {
	t.Parallel()
	env := startupEnv()
	collector := &startupCollector{running: make(chan struct{}), stopped: make(chan struct{})}
	env.Initialize = func(ctx context.Context) (Sources, error) {
		env.Scheduler.Every(collector, time.Hour)
		env.Scheduler.Prime(ctx, 2, 0)
		return Sources{}, nil
	}
	root := New(env)
	app := reactea.New(modal.New(root), reactea.WithRoute("/graphs/cpu"), reactea.WithSize(100, 28))
	t.Cleanup(app.Scope().Close)
	_ = app.Init()
	command := root.initialize(app.Ctx().Context())
	app.Send(command())
	select {
	case <-collector.running:
	case <-time.After(5 * time.Second):
		t.Fatal("regular collection did not start after the two initial readings")
	}
	// The real runner's quit filter closes this scope; App.Send does not run
	// that filter. The rendering test above covers quitting through the runner.
	app.Scope().Close()
	select {
	case <-collector.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("closing the UI did not cancel regular collection")
	}
}

func TestRunCancelsInitializationWhenItsContextEnds(t *testing.T) {
	t.Parallel()
	env := startupEnv()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan struct{})
	env.Initialize = func(ctx context.Context) (Sources, error) {
		cancel()
		<-ctx.Done()
		close(stopped)
		return Sources{}, ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env, tea.WithInput(nil), tea.WithOutput(io.Discard),
			tea.WithoutRenderer(), tea.WithoutSignalHandler(), tea.WithWindowSize(100, 28))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("program ignored cancellation during startup")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("program exit did not close the initializer's scope")
	}
}
