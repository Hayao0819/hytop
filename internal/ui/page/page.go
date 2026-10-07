// Package page builds a page out of widgets.
package page

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

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
		statgrid.Stat{Label: "zram stored", Key: "mem.zram.original", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "zram compressed", Key: "mem.zram.compressed", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "zram memory", Key: "mem.zram.used", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "zram ratio", Key: "mem.zram.ratio", Unit: series.None, Precision: 2},
		statgrid.Stat{Label: "zswap stored", Key: "mem.zswap.stored", Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "zswap compressed", Key: "mem.zswap.compressed", Unit: series.Bytes, Precision: 1},
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
		statgrid.Stat{Label: "Memory clock", Key: series.Key(prefix + "mem.clock"), Unit: series.Hertz, Precision: 2},
		statgrid.Stat{Label: "GTT memory", Key: series.Key(prefix + "mem.gtt.used"), Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "GTT total", Key: series.Key(prefix + "mem.gtt.total"), Unit: series.Bytes, Precision: 1},
		statgrid.Stat{Label: "Encoder", Key: series.Key(prefix + "encode"), Unit: series.Percent},
		statgrid.Stat{Label: "Decoder", Key: series.Key(prefix + "decode"), Unit: series.Percent},
		statgrid.Stat{Label: "Fan", Key: series.Key(prefix + "fan"), Unit: series.Percent},
		statgrid.Stat{Label: "Fan speed", Key: series.Key(prefix + "fan.rpm"), Unit: series.RPM},
		statgrid.Stat{Label: "PCIe link", Fact: prefix + "pcie.current"},
		statgrid.Stat{Label: "PCIe maximum", Fact: prefix + "pcie.max"},
		statgrid.Stat{Label: "Card", Fact: fmt.Sprintf("gpu.name.%d", index)},
	)

	return Detail(env, name, g, stats)
}

func Battery(env Env, index int, name string) reactea.Component {
	prefix := fmt.Sprintf("battery.%d.", index)
	g := graph.New(env.Store, env.Caps,
		graph.Track{Key: series.Key(prefix + "capacity"), Colour: env.Theme.Colour(theme.Battery)},
	)
	g.Range = chart.Range{Max: 100}
	g.Label = "% charge"

	stats := statgrid.New(env.Store,
		statgrid.Stat{Label: "Charge", Key: series.Key(prefix + "capacity"), Unit: series.Percent, Big: true},
		statgrid.Stat{Label: "Health", Key: series.Key(prefix + "health"), Unit: series.Percent, Big: true},
		statgrid.Stat{Label: "Power", Key: series.Key(prefix + "power"), Unit: series.Watts, Precision: 1, Big: true},
		statgrid.Stat{Label: "Remaining", Key: series.Key(prefix + "time_to_empty"), Unit: series.Duration, Big: true},
		statgrid.Stat{Label: "Until full", Key: series.Key(prefix + "time_to_full"), Unit: series.Duration, Big: true},
		statgrid.Stat{Label: "Energy", Key: series.Key(prefix + "energy"), Unit: series.WattHours, Precision: 1},
		statgrid.Stat{Label: "Full capacity", Key: series.Key(prefix + "energy_full"), Unit: series.WattHours, Precision: 1},
		statgrid.Stat{Label: "Design capacity", Key: series.Key(prefix + "energy_design"), Unit: series.WattHours, Precision: 1},
		statgrid.Stat{Label: "Voltage", Key: series.Key(prefix + "voltage"), Unit: series.Volts, Precision: 2},
		statgrid.Stat{Label: "Temperature", Key: series.Key(prefix + "temp"), Unit: series.Celsius, Precision: 1},
		statgrid.Stat{Label: "Cycles", Key: series.Key(prefix + "cycles"), Unit: series.Count},
		statgrid.Stat{Label: "Start charging", Key: series.Key(prefix + "charge_start"), Unit: series.Percent},
		statgrid.Stat{Label: "Stop charging", Key: series.Key(prefix + "charge_end"), Unit: series.Percent},
	)

	facts := statgrid.New(env.Store,
		statgrid.Stat{Label: "Status", Fact: "battery.status." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Condition", Fact: "battery.health_status." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Manufacturer", Fact: "battery.vendor." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Model", Fact: "battery.model." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Serial", Fact: "battery.serial." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Technology", Fact: "battery.technology." + strconv.Itoa(index)},
		statgrid.Stat{Label: "Mains", Fact: series.FactACOnline},
	)

	page := DetailWithFacts(env, name, g, stats, facts)
	g.Span = 7 * 24 * time.Hour

	return page
}
