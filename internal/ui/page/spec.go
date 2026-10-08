package page

import (
	"runtime"
	"strings"
	"time"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// Env contains dependencies shared by pages.
type Env struct {
	Store    *store.Store
	View     *store.ViewState
	Registry *series.Registry
	Caps     render.Caps
	Theme    *theme.Theme
	Keys     *keymap.Map
	Span     time.Duration

	Columns []procmodel.Column

	Interfaces func() []string
	// GPUs includes identifiable cards even when no counters are available.
	GPUs func() []int
	// Batteries includes power-supply batteries even when optional readings are absent.
	Batteries func() []int
}

// Hint is one key-line entry.
type Hint = keymap.Hint

// Hinter provides state-dependent key hints.
type Hinter interface {
	Hints() []Hint
}

// Mode describes a top-level tab and its routes.
type Mode struct {
	Title string

	// Slot indexes the configurable mode-key binding.
	Slot int

	// Right aligns the tab to the opposite end of the bar.
	Right bool

	// Home is the default route; Prefix includes child routes in the mode.
	Home   string
	Prefix string
}

// Holds reports whether route belongs to the mode.
func (m Mode) Holds(route string) bool {
	return route == m.Home || (m.Prefix != "" && len(route) >= len(m.Prefix) && route[:len(m.Prefix)] == m.Prefix)
}

// Modes returns the platform's built-in top-level modes.
func Modes() []Mode {
	modes := []Mode{
		{Title: "Graphs", Home: "/graphs/cpu", Prefix: "/graphs/"},
		{Title: "Processes", Home: "/processes"},
	}

	if runtime.GOOS == "linux" {
		modes = append(modes, Mode{Title: "Services", Home: "/services"})
	}

	modes = append(modes, Mode{Title: "Disk", Home: "/disk/filesystems", Prefix: "/disk/"})
	if runtime.GOOS == "linux" {
		modes = append(modes, Mode{Title: "Containers", Home: "/containers"})
	}

	modes = append(modes, Mode{Title: "Settings", Right: true, Home: "/settings"})
	for i := range modes {
		modes[i].Slot = i
	}
	return modes
}

// GraphSpec describes one device-rail entry and its page.
type GraphSpec struct {
	Title string
	Route string
	Build func(Env) reactea.Component
	// Sources remount a dynamic page when its inputs change.
	Sources []series.Key

	Key   series.Key
	Unit  series.Unit
	Token theme.Token
	Max   float64
}

// GraphSpecs returns the built-in performance pages.
func GraphSpecs() []GraphSpec {
	return []GraphSpec{
		{
			Title: "CPU", Route: "/graphs/cpu", Build: CPU,
			Key: "cpu.total.usage", Unit: series.Percent, Token: theme.CPU, Max: 100,
		},
		{
			Title: "Memory", Route: "/graphs/memory", Build: Memory,
			Key: "mem.usage", Unit: series.Percent, Token: theme.Memory, Max: 100,
		},
		{
			Title: "Disk", Route: "/graphs/disk", Build: Disk,
			Key: "diskio.total.read", Unit: series.BytesPerSecond, Token: theme.DiskRead,
		},
		{
			Title: "Network", Route: "/graphs/network", Build: Network,
			Key: "net.total.rx", Unit: series.BytesPerSecond, Token: theme.NetRx,
		},
		{
			Title: "Sensors", Route: "/graphs/sensors", Build: Sensors,
			Key: "thermal.max.temp", Unit: series.Celsius, Token: theme.Critical, Max: 100,
		},
	}
}

// Available removes specifications whose primary series is absent.
func Available(specs []GraphSpec, has func(series.Key) bool) []GraphSpec {
	kept := make([]GraphSpec, 0, len(specs))

	for _, spec := range specs {
		if spec.Key == "" || has(spec.Key) {
			kept = append(kept, spec)
		}
	}

	if len(kept) == 0 {
		return specs[:1]
	}

	return kept
}

func line(hints []Hint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, hint.Key+" "+hint.What)
	}

	return strings.Join(parts, "   ")
}
