package page_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func graphProgram(memory *store.Store, cards, batteries func() []int, route string) *reactea.App {
	caps := render.Caps{Glyphs: render.Block, Colors: render.Mono}
	graphs := page.NewGraphs(page.Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Caps: caps, Theme: theme.Build(caps, nil), Keys: keymap.Default(), Span: time.Minute,
		Interfaces: func() []string { return nil }, GPUs: cards, Batteries: batteries,
	})
	program := reactea.New(graphs, reactea.WithSize(80, 24), reactea.WithRoute(route))
	program.Start()

	return program
}

func TestRemovingTheCurrentGPUReturnsToAnAvailableGraph(t *testing.T) {
	memory := store.New(metric.DefaultResolutions())
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "External GPU"})
	cards := []int{0}
	program := graphProgram(memory, func() []int { return cards }, nil, "/graphs/gpu/0")

	if got := testkit.Plain(program); !strings.Contains(got, "External GPU") {
		t.Fatalf("GPU page did not open:\n%s", got)
	}

	cards = nil
	memory.ReplaceFacts("gpu", nil)
	program.Send(struct{}{})

	if program.Route() != "/graphs/cpu" {
		t.Fatalf("route after removing current GPU = %q, want /graphs/cpu", program.Route())
	}
	if got := testkit.Plain(program); strings.Contains(got, "External GPU") {
		t.Fatalf("removed GPU remained on screen:\n%s", got)
	}
}

func TestRenamingTheCurrentGPURemountsItsPage(t *testing.T) {
	memory := store.New(metric.DefaultResolutions())
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "Old GPU"})
	program := graphProgram(memory, func() []int { return []int{0} }, nil, "/graphs/gpu/0")

	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "New GPU"})
	program.Send(struct{}{})

	got := testkit.Plain(program)
	if !strings.Contains(got, "New GPU") || strings.Contains(got, "Old GPU") {
		t.Fatalf("renamed GPU page was not rebuilt:\n%s", got)
	}
}

func TestBatteryGetsAWeeklyPageAndDisappearsAfterRemoval(t *testing.T) {
	memory := store.New(metric.ResolutionsFor(7 * 24 * time.Hour))
	now := time.Now()
	samples := make([]metric.Sample, 0, 169)
	for hour := 168; hour >= 0; hour-- {
		samples = append(samples, metric.Sample{
			Key: "battery.0.capacity", Value: float64(75 - hour%10),
			Time: now.Add(-time.Duration(hour) * time.Hour),
		})
	}
	memory.WriteSamples(samples)
	memory.ReplaceFacts("power", map[string]string{"battery.name.0": "Internal battery"})
	batteries := []int{0}
	program := graphProgram(memory, nil, func() []int { return batteries }, "/graphs/battery/0")

	if got := testkit.Plain(program); !strings.Contains(got, "Internal battery") || !strings.Contains(got, "% charge") {
		t.Fatalf("battery page did not show its name and charge history:\n%s", got)
	}

	batteries = nil
	memory.ReplaceFacts("power", nil)
	program.Send(struct{}{})
	if program.Route() != "/graphs/cpu" {
		t.Fatalf("route after removing battery = %q, want /graphs/cpu", program.Route())
	}
}
