// Package page builds a page out of widgets.
package page

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/graph"
	"github.com/Hayao0819/hytop/internal/ui/widget/statgrid"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Explicit border sides trigger charmbracelet/lipgloss#732.
var pane = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).Padding(0, 1)

// minGraph reserves two border rows and four plot rows.
const minGraph = 6

type detail struct {
	reactea.Wrapper

	box    *layout.Box
	title  string
	head   layout.Item
	graph  layout.Item
	stats  *statgrid.Widget
	facts  *statgrid.Widget
	shaped [2]int
}

// Detail builds a graph with live statistics.
func Detail(env Env, title string, g *graph.Widget, stats *statgrid.Widget) reactea.Component {
	return DetailWithFacts(env, title, g, stats, nil)
}

// DetailWithFacts adds a block for stable readings below the live statistics.
func DetailWithFacts(
	env Env, title string, g *graph.Widget, stats, facts *statgrid.Widget,
) reactea.Component {
	g.Span = env.Span

	d := &detail{title: title, stats: stats, facts: facts}

	d.head = layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
		return lipgloss.NewStyle().Bold(true).Width(ctx.Width()).Render(title)
	}))

	active := pane.BorderForeground(env.Theme.Colour(theme.BorderActive))
	d.graph = layout.Grow(3, layout.Memo(
		layout.Framed(pane.BorderForeground(env.Theme.Colour(theme.Border)), g).WhenFocused(active),
		func() any { return env.Store.Generation() },
	)).Bounds(minGraph, 0).Focusable()

	d.box = layout.Column(d.head, d.graph)
	d.reshape()

	d.Wrapper = reactea.Wrap(d.box)

	return d
}

// Update adjusts dynamic statistic and fact rows before updating children.
func (d *detail) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	d.reshape()

	return d.Wrapper.Update(ctx, msg)
}

func (d *detail) reshape() {
	rows := [2]int{d.stats.Rows(), 0}
	if d.facts != nil {
		rows[1] = d.facts.Rows()
	}

	if rows == d.shaped {
		return
	}

	d.shaped = rows

	items := []layout.Item{d.head, d.graph}
	if rows[0] > 0 {
		items = append(items, layout.Grow(1, d.stats).Bounds(min(2, rows[0]), rows[0]))
	}

	if rows[1] > 0 {
		items = append(items, layout.Grow(1, d.facts).Bounds(min(2, rows[1]), rows[1]))
	}

	d.box.SetItems(items...)
}

// Hints reports when the terminal is too short for the graph.
func (d *detail) Hints() []Hint {
	if len(d.box.Starved()) == 0 {
		return nil
	}

	return []Hint{{Key: "!", What: "the window is too short for this graph"}}
}

func Memory(env Env) reactea.Component {
	g := graph.New(env.Store, env.Caps,
		graph.Track{Key: "mem.usage", Colour: env.Theme.Colour(theme.Memory)},
	)
	g.Range = chart.Range{Max: 100}
	g.Label = "% memory in use"

	stats := statgrid.New(env.Store,
		statgrid.Stat{Label: "In use", Key: "mem.used", Unit: series.Bytes, Precision: 1, Big: true},
		statgrid.Stat{Label: "Available", Key: "mem.available", Unit: series.Bytes, Precision: 1, Big: true},
		statgrid.Stat{Label: "Cached", Key: "mem.cached", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Total", Key: "mem.total", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Swap used", Key: "swap.used", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Swap total", Key: "swap.total", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Pressure 10s", Key: "psi.memory.some.avg10", Unit: series.Percent, Precision: 2},
		statgrid.Stat{Label: "Pressure 60s", Key: "psi.memory.some.avg60", Unit: series.Percent, Precision: 2},
	)

	slots := statgrid.New(env.Store,
		statgrid.Stat{Label: "Speed", Fact: series.FactMemorySpeed},
		statgrid.Stat{Label: "Slots used", Fact: series.FactMemorySlotsUsed},
		statgrid.Stat{Label: "Slots", Fact: series.FactMemorySlots},
		statgrid.Stat{Label: "Form factor", Fact: series.FactMemoryForm},
		statgrid.Stat{Label: "Type", Fact: series.FactMemoryType},
		statgrid.Stat{Label: "Modules", Fact: series.FactMemoryModules},
		statgrid.Stat{Label: "Max capacity", Fact: series.FactMemoryMaxCapacity},
		statgrid.Stat{Label: "Part number", Fact: series.FactMemoryPart},
	)

	return DetailWithFacts(env, "Memory", g, stats, slots)
}

func Disk(env Env) reactea.Component {
	g := graph.New(env.Store, env.Caps,
		graph.Track{Key: "diskio.total.read", Colour: env.Theme.Colour(theme.DiskRead)},
		graph.Track{Key: "diskio.total.write", Colour: env.Theme.Colour(theme.DiskWrite)},
	)
	g.Range = chart.Range{}
	g.Unit = series.BytesPerSecond
	g.Fill = false
	g.Label = "read and write"

	stats := statgrid.New(env.Store,
		statgrid.Stat{Label: "Read", Key: "diskio.total.read", Unit: series.BytesPerSecond, Precision: 1, Big: true},
		statgrid.Stat{Label: "Write", Key: "diskio.total.write", Unit: series.BytesPerSecond, Precision: 1, Big: true},
		statgrid.Stat{Label: "IO pressure 10s", Key: "psi.io.some.avg10", Unit: series.Percent, Precision: 2},
		statgrid.Stat{Label: "IO pressure 60s", Key: "psi.io.some.avg60", Unit: series.Percent, Precision: 2},
	)

	return Detail(env, "Disk", g, stats)
}

// byCoreNumber sorts cpu.core.10 after cpu.core.9, which sorting the keys as
// text does not.
func byCoreNumber(keys []series.Key) []series.Key {
	sorted := slices.Clone(keys)

	slices.SortFunc(sorted, func(a, b series.Key) int {
		return cmp.Compare(coreNumber(a), coreNumber(b))
	})

	return sorted
}

func coreNumber(key series.Key) int {
	segments := key.Segments()
	if len(segments) < 3 {
		return -1
	}

	n, err := strconv.Atoi(segments[2])
	if err != nil {
		return -1
	}

	return n
}

func Sensors(env Env) reactea.Component {
	g := graph.New(env.Store, env.Caps,
		graph.Track{Key: "thermal.max.temp", Colour: env.Theme.Colour(theme.Critical)},
		graph.Track{Key: "cpu.package.temp", Colour: env.Theme.Colour(theme.CPU)},
	)
	g.Range = chart.Range{Max: 100}
	g.Unit = series.Celsius
	g.Label = "hottest sensor, with the cpu package over it"

	stats := statgrid.New(env.Store,
		statgrid.Stat{Label: "Hottest", Key: "thermal.max.temp", Unit: series.Celsius, Big: true},
		statgrid.Stat{Label: "CPU package", Key: "cpu.package.temp", Unit: series.Celsius, Big: true},
		statgrid.Stat{Label: "Battery", Key: "battery.0.capacity", Unit: series.Percent},
		statgrid.Stat{Label: "Battery draw", Key: "battery.0.power", Unit: series.Watts, Precision: 1},
		statgrid.Stat{Label: "Mains", Fact: "power.ac"},
		statgrid.Stat{Label: "Battery status", Fact: "battery.status"},
		statgrid.Stat{Label: "Battery health", Fact: "battery.health"},
	)

	return Detail(env, "Sensors", g, stats)
}

func GPU(env Env, index int, name string) reactea.Component {
	prefix := fmt.Sprintf("gpu.%d.", index)
	g := graph.New(env.Store, env.Caps,
		graph.Track{Key: series.Key(prefix + "util"), Colour: env.Theme.Colour(theme.CPU)},
	)
	g.Range = chart.Range{Max: 100}
	g.Label = "% utilisation"

	stats := statgrid.New(env.Store,
		statgrid.Stat{Label: "Utilisation", Key: series.Key(prefix + "util"), Unit: series.Percent, Precision: 1, Big: true},
		statgrid.Stat{Label: "Memory", Key: series.Key(prefix + "mem.used"), Unit: series.Bytes, Precision: 1, Big: true},
		statgrid.Stat{Label: "Temperature", Key: series.Key(prefix + "temp"), Unit: series.Celsius, Big: true},
		statgrid.Stat{Label: "Power", Key: series.Key(prefix + "power"), Unit: series.Watts, Precision: 1, Big: true},
		statgrid.Stat{Label: "Memory total", Key: series.Key(prefix + "mem.total"), Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Clock", Key: series.Key(prefix + "clock"), Unit: series.Hertz, Precision: 2},
		statgrid.Stat{Label: "Card", Fact: fmt.Sprintf("gpu.name.%d", index)},
	)

	return Detail(env, name, g, stats)
}
