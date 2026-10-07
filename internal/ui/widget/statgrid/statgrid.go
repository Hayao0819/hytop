// Package statgrid draws compact label-and-value cells below a graph.
package statgrid

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
)

// Stat describes one label-and-value cell. Key takes precedence over Text.
type Stat struct {
	Label     string
	Key       series.Key
	Unit      series.Unit
	Precision int
	Text      string
	Fact      string
	Big       bool

	// Always retains the cell when neither Key nor Text has a value.
	Always bool
}

type Widget struct {
	reactea.BasicComponent

	store   *store.Store
	stats   []Stat
	Columns int
}

func New(s *store.Store, stats ...Stat) *Widget {
	return &Widget{store: s, stats: stats, Columns: 4}
}

// Rows returns the height after unavailable readings are omitted. When width
// is known, it accounts for the same responsive column count as Render.
func (w *Widget) Rows(width ...int) int {
	present := len(w.present())
	if present == 0 {
		return 0
	}

	columns := max(1, w.Columns)
	if len(width) > 0 {
		columns = max(1, min(columns, width[0]/18))
	}

	return 2 * ((present + columns - 1) / columns)
}

// present is the cells worth a place. A reading this machine cannot produce
// leaves no gap: the ones after it move up.
func (w *Widget) present() []Stat {
	kept := make([]Stat, 0, len(w.stats))

	for _, stat := range w.stats {
		if stat.Always || w.has(stat) {
			kept = append(kept, stat)
		}
	}

	return kept
}

func (w *Widget) has(stat Stat) bool {
	switch {
	case stat.Text != "":
		return true
	case stat.Fact != "":
		_, ok := w.store.Fact(stat.Fact)

		return ok
	default:
		_, ok := w.store.Last(stat.Key)

		return ok
	}
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	var (
		stats   = w.present()
		columns = max(1, min(w.Columns, width/18))
		cellW   = width / columns
		label   = lipgloss.NewStyle().Faint(true)
		big     = lipgloss.NewStyle().Bold(true)
		lines   []string
	)

	for i := 0; i < len(stats); i += columns {
		var labels, values strings.Builder

		for _, stat := range stats[i:min(i+columns, len(stats))] {
			plainValue := w.value(stat)
			value := plainValue
			if stat.Big {
				value = big.Render(value)
			}

			labels.WriteString(uirender.Left(label.Render(stat.Label), cellW))
			values.WriteString(uirender.Left(value, cellW))
		}

		lines = append(lines, labels.String(), values.String())

		if len(lines) >= height {
			break
		}
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (w *Widget) value(stat Stat) string {
	if stat.Text != "" {
		return stat.Text
	}

	if stat.Fact != "" {
		if value, ok := w.store.Fact(stat.Fact); ok {
			return value
		}

		return "—"
	}

	point, ok := w.store.Last(stat.Key)
	if !ok {
		return "—"
	}

	return stat.Unit.Format(point.Value, stat.Precision, series.Auto)
}
