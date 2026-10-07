package app_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

func bare(t *testing.T, cards func() []string) (*reactea.App, *store.Store) {
	t.Helper()

	memory := store.New(metric.DefaultResolutions())

	settings := conf.Default()
	settings.General.Interval = conf.Duration(time.Millisecond)

	var registry series.Registry

	root := app.New(app.Env{
		Store:      memory,
		View:       store.NewViewState(),
		Registry:   &registry,
		Scheduler:  collect.NewScheduler(memory),
		Caps:       render.Caps{Glyphs: render.Braille, Colors: render.Ansi256},
		Config:     settings,
		Interfaces: cards,
	})

	program := reactea.New(modal.New(root), reactea.WithSize(100, 28), reactea.WithRoute("/graphs/cpu"))

	_ = program.Init()

	return program, memory
}

func readings(memory *store.Store, keys ...series.Key) {
	now := time.Now()

	samples := make([]metric.Sample, 0, len(keys)*4)

	for _, key := range keys {
		for i := range 4 {
			samples = append(samples, metric.Sample{
				Key: key, Value: float64(i + 1), Time: now.Add(time.Duration(i-4) * time.Second),
			})
		}
	}

	memory.WriteSamples(samples)
}

func TestTheRailFollowsWhatHasBeenRead(t *testing.T) {
	t.Parallel()

	program, memory := bare(t, nil)

	if got := plain(program); strings.Contains(got, "Memory") || strings.Contains(got, "Network") {
		t.Fatalf("the rail names devices nothing has been read from:\n%s", got)
	}

	readings(memory, "cpu.total.usage", "mem.usage", "diskio.total.read", "net.total.rx")

	press(t, program, "j")

	got := plain(program)

	for _, want := range []string{"CPU", "Memory", "Disk", "Network"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q never joined the rail once it had readings:\n%s", want, got)
		}
	}

	if strings.Contains(got, "GPU") {
		t.Errorf("the rail lists a device with no readings:\n%s", got)
	}
}

func TestADeviceThatArrivesBecomesReachable(t *testing.T) {
	t.Parallel()

	program, memory := bare(t, nil)

	readings(memory, "cpu.total.usage", "mem.usage")
	press(t, program, "j")

	if got := program.Route(); got != "/graphs/memory" {
		t.Fatalf("route = %q, want the memory page the rail now offers", got)
	}

	readings(memory, "thermal.max.temp")

	for range 4 {
		press(t, program, "j")
	}

	if got := program.Route(); got != "/graphs/sensors" {
		t.Errorf("route = %q; a device added after the page was built is not walkable", got)
	}

	if got := plain(program); !strings.Contains(got, "Hottest") {
		t.Errorf("the sensors page did not open:\n%s", got)
	}
}

type cards struct {
	mu    sync.Mutex
	found []string
}

func (c *cards) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.found...)
}

func (c *cards) set(names ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.found = names
}

func TestNetworkPanesFollowTheInterfaceList(t *testing.T) {
	t.Parallel()

	found := &cards{}

	program, memory := bare(t, found.list)

	readings(memory, "cpu.total.usage", "net.total.rx")
	press(t, program, "j")

	if got := program.Route(); got != "/graphs/network" {
		t.Fatalf("route = %q, want the network page", got)
	}

	if got := plain(program); !strings.Contains(got, "no interfaces with a link") {
		t.Fatalf("with no cards the page should say so:\n%s", got)
	}

	found.set("eth0")
	tick(program)

	if got := plain(program); !strings.Contains(got, "eth0") {
		t.Errorf("a card that turned up while the page was open is missing:\n%s", got)
	}

	found.set("eth0", "wlan0")
	tick(program)

	if got := plain(program); !strings.Contains(got, "wlan0") {
		t.Errorf("a second card did not get a pane:\n%s", got)
	}
}
