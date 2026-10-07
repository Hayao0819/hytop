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

func graphProgram(memory *store.Store, cards func() []int, route string) *reactea.App {
	caps := render.Caps{Glyphs: render.Block, Colors: render.Mono}
	graphs := page.NewGraphs(page.Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Caps: caps, Theme: theme.Build(caps, nil), Keys: keymap.Default(), Span: time.Minute,
		Interfaces: func() []string { return nil }, GPUs: cards,
	})
	program := reactea.New(graphs, reactea.WithSize(80, 24), reactea.WithRoute(route))
	program.Start()

	return program
}

func TestRemovingTheCurrentGPUReturnsToAnAvailableGraph(t *testing.T) {
	memory := store.New(metric.DefaultResolutions())
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "External GPU"})
	cards := []int{0}
	program := graphProgram(memory, func() []int { return cards }, "/graphs/gpu/0")

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
	program := graphProgram(memory, func() []int { return []int{0} }, "/graphs/gpu/0")

	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "New GPU"})
	program.Send(struct{}{})

	got := testkit.Plain(program)
	if !strings.Contains(got, "New GPU") || strings.Contains(got, "Old GPU") {
		t.Fatalf("renamed GPU page was not rebuilt:\n%s", got)
	}
}
