package page

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"
	"github.com/Hayao0819/reactea/v2/router"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/devicelist"
)

// Graphs displays the device rail and selected performance page.
type Graphs struct {
	reactea.Wrapper

	env   Env
	keys  *keymap.Map
	specs []GraphSpec
	rail  *devicelist.Widget
	pages *router.Component

	// generation tracks when dynamic device discovery must run again.
	generation uint64
}

func NewGraphs(env Env) *Graphs {
	g := &Graphs{env: env, keys: env.Keys}

	g.pages = router.NewWithRoutes(router.Routes{})
	g.rail = devicelist.New(env.Store, env.Caps, env.Theme)

	g.refresh()

	g.Wrapper = reactea.Wrap(layout.Row(
		layout.Grow(1, g.rail).Bounds(16, 22),
		layout.Spacer(1),
		verticalRule(env.Theme),
		layout.Spacer(1),
		layout.Grow(4, g.pages).Focusable(),
	))

	return g
}

func (g *Graphs) refresh() bool {
	specs := Available(GraphSpecs(), func(key series.Key) bool {
		_, ok := g.env.Store.Last(key)

		return ok
	})
	var gpuIndices []int
	if g.env.GPUs != nil {
		gpuIndices = g.env.GPUs()
	}
	var batteryIndices []int
	if g.env.Batteries != nil {
		batteryIndices = g.env.Batteries()
	}
	if keys := powerSeries(g.env.Store); len(batteryIndices) == 0 && len(keys) > 0 {
		specs = append(specs, GraphSpec{
			Title: "Power", Route: "/graphs/power", Key: keys[0],
			Unit: series.Watts, Token: theme.CPU, Sources: keys,
			Build: func(env Env) reactea.Component { return Power(env, keys) },
		})
	}
	for _, index := range gpuIndices {
		index := index
		key := series.Key(fmt.Sprintf("gpu.%d.util", index))
		name, ok := g.env.Store.Fact(fmt.Sprintf("gpu.name.%d", index))
		if !ok || name == "" {
			name = fmt.Sprintf("GPU %d", index)
		}

		specs = append(specs, GraphSpec{
			Title: name, Route: fmt.Sprintf("/graphs/gpu/%d", index), Key: key,
			Unit: series.Percent, Token: theme.Memory, Max: 100,
			Build: func(env Env) reactea.Component { return GPU(env, index, name) },
		})
	}
	for _, index := range batteryIndices {
		index := index
		key := series.Key(fmt.Sprintf("battery.%d.capacity", index))
		name, ok := g.env.Store.Fact(fmt.Sprintf("battery.name.%d", index))
		if !ok || name == "" {
			name = fmt.Sprintf("Battery %d", index)
		}

		specs = append(specs, GraphSpec{
			Title: name, Route: fmt.Sprintf("/graphs/battery/%d", index), Key: key,
			Unit: series.Percent, Token: theme.Battery, Max: 100,
			Build: func(env Env) reactea.Component { return Battery(env, index, name) },
		})
	}

	if same(specs, g.specs) {
		return false
	}

	g.specs = specs

	routes := router.Routes{"default": func(router.Params) reactea.Component { return CPU(g.env) }}
	entries := make([]devicelist.Entry, 0, len(specs))

	for _, spec := range specs {
		build := spec.Build
		routes[spec.Route] = func(router.Params) reactea.Component { return build(g.env) }

		entries = append(entries, devicelist.Entry{
			Title:  spec.Title,
			Route:  spec.Route,
			Key:    spec.Key,
			Unit:   spec.Unit,
			Colour: g.env.Theme.Colour(spec.Token),
			Max:    spec.Max,
		})
	}

	g.pages.SetRoutes(routes, router.RemountCurrent)
	g.rail.SetEntries(entries)

	return true
}

func same(a, b []GraphSpec) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Route != b[i].Route || a[i].Title != b[i].Title || !slices.Equal(a[i].Sources, b[i].Sources) {
			return false
		}
	}

	return true
}

// Hints returns detail-page hints when available, then graph navigation hints.
func (g *Graphs) Hints() []Hint {
	if hinter, ok := g.pages.Current().(Hinter); ok {
		if hints := hinter.Hints(); hints != nil {
			return hints
		}
	}

	return g.keys.Hints(keymap.Graphs)
}

func (g *Graphs) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if generation := g.env.Store.Generation(); generation != g.generation {
		g.generation = generation

		if g.refresh() && !g.hasRoute(ctx.Route()) {
			return tea.Batch(ctx.SetRoute(g.specs[0].Route), g.Wrapper.Update(ctx, msg))
		}
	}

	switch {
	case g.keys.Is(msg, keymap.Graphs, keymap.Down):
		return g.step(ctx, 1)
	case g.keys.Is(msg, keymap.Graphs, keymap.Up):
		return g.step(ctx, -1)
	}

	return g.Wrapper.Update(ctx, msg)
}

func (g *Graphs) hasRoute(route string) bool {
	for _, spec := range g.specs {
		if spec.Route == route {
			return true
		}
	}

	return false
}

func (g *Graphs) step(ctx *reactea.Ctx, by int) tea.Cmd {
	current := 0

	for i, spec := range g.specs {
		if spec.Route == ctx.Route() {
			current = i
		}
	}

	next := (current + by + len(g.specs)) % len(g.specs)

	return ctx.SetRoute(g.specs[next].Route)
}
