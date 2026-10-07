package page

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/coregrid"
	"github.com/Hayao0819/hytop/internal/ui/widget/graph"
	"github.com/Hayao0819/hytop/internal/ui/widget/statgrid"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// CPU builds the aggregate/per-core CPU performance page.
func CPU(env Env) reactea.Component {
	return newCPU(env)
}

type cpuPage struct {
	reactea.Wrapper

	env  Env
	box  *layout.Box
	one  reactea.Component
	many reactea.Component
	grid bool
}

func newCPU(env Env) *cpuPage {
	p := &cpuPage{env: env}
	active := pane.BorderForeground(env.Theme.Colour(theme.BorderActive))
	quiet := pane.BorderForeground(env.Theme.Colour(theme.Border))

	total := graph.New(env.Store, env.Caps,
		graph.Track{Key: "cpu.total.usage", Colour: env.Theme.Colour(theme.CPU)},
		graph.Track{Key: "psi.cpu.some.avg10", Colour: env.Theme.Colour(theme.CPUPressure)},
	)
	total.Range = chart.Range{Max: 100}
	total.Span = env.Span
	total.Label = "% utilisation, with cpu pressure over it"

	p.one = layout.Framed(quiet, total).WhenFocused(active)

	cores := coregrid.New(env.Store, env.Caps, env.Theme.Colour(theme.CPU), func() []series.Key {
		return byCoreNumber(series.Pattern("cpu.core.*.usage").Expand(env.Store.Keys()))
	})
	cores.Span = env.Span
	cores.Border = env.Theme.Colour(theme.Border)

	p.many = layout.Framed(quiet, cores).WhenFocused(active)

	p.box = layout.Column(
		layout.Fixed(1, reactea.Func(p.heading)),
		layout.Grow(3, p.one).Bounds(minGraph, 0).Key("plot").Focusable(),
		layout.Grow(2, cpuStats(env)).Bounds(4, 6),
		layout.Grow(1, cpuFacts(env)).Bounds(4, 4),
	)

	p.Wrapper = reactea.Wrap(p.box)

	return p
}

func (p *cpuPage) Hints() []Hint {
	if len(p.box.Starved()) > 0 {
		return []Hint{{Key: "!", What: "the window is too short for this page"}}
	}

	return p.env.Keys.Hints(keymap.CPU)
}

func (p *cpuPage) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if p.env.Keys.Is(msg, keymap.CPU, keymap.PerCore) {
		p.grid = !p.grid

		plot := p.one
		if p.grid {
			plot = p.many
		}

		p.box.Replace("plot", plot)

		return nil
	}

	return p.Wrapper.Update(ctx, msg)
}

func (p *cpuPage) heading(ctx *reactea.Ctx) string {
	title := "CPU"
	if model, ok := p.env.Store.Fact(series.FactCPUModel); ok {
		title += "   " + model
	}

	return lipgloss.NewStyle().Bold(true).Width(ctx.Width()).MaxWidth(ctx.Width()).Render(title)
}

func cpuStats(env Env) *statgrid.Widget {
	return statgrid.New(env.Store,
		statgrid.Stat{Label: "Utilisation", Key: "cpu.total.usage", Unit: series.Percent, Precision: 1, Big: true},
		statgrid.Stat{Label: "Speed", Key: "cpu.total.freq", Unit: series.Hertz, Precision: 2, Big: true},
		statgrid.Stat{Label: "Temperature", Key: "cpu.package.temp", Unit: series.Celsius, Big: true},
		statgrid.Stat{Label: "Package power", Key: "cpu.package.power", Unit: series.Watts, Precision: 1, Big: true},
		statgrid.Stat{Label: "Up time", Key: "system.uptime", Unit: series.Duration, Big: true},
		statgrid.Stat{Label: "Processes", Key: "proc.count", Unit: series.Count},
		statgrid.Stat{Label: "Threads", Key: "proc.threads", Unit: series.Count},
		statgrid.Stat{Label: "Handles", Key: "proc.fds", Unit: series.Count},
		statgrid.Stat{Label: "Pressure 10s", Key: "psi.cpu.some.avg10", Unit: series.Percent, Precision: 2},
	)
}

func cpuFacts(env Env) *statgrid.Widget {
	return statgrid.New(env.Store,
		statgrid.Stat{Label: "Base speed", Fact: series.FactCPUBaseSpeed},
		statgrid.Stat{Label: "Sockets", Fact: series.FactCPUSockets},
		statgrid.Stat{Label: "Cores", Fact: series.FactCPUCores},
		statgrid.Stat{Label: "Logical processors", Fact: series.FactCPULogical},
		statgrid.Stat{Label: "Virtualisation", Fact: series.FactCPUVirtualisation},
		statgrid.Stat{Label: "L1 cache", Fact: series.FactCPUL1d},
		statgrid.Stat{Label: "L2 cache", Fact: series.FactCPUL2},
		statgrid.Stat{Label: "L3 cache", Fact: series.FactCPUL3},
	)
}
