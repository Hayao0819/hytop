// Package netgraph draws one interface: receive above the line, send below.
package netgraph

import (
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/widget"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

type Widget struct {
	reactea.BasicComponent

	store *store.Store
	caps  render.Caps
	iface string

	Rx, Tx color.Color
	Span   time.Duration
}

func New(s *store.Store, caps render.Caps, iface string, rx, tx color.Color) *Widget {
	return &Widget{store: s, caps: caps, iface: iface, Rx: rx, Tx: tx, Span: time.Minute}
}

func (w *Widget) key(what string) series.Key {
	return series.Key("net." + series.NormalizeSegment(w.iface) + "." + what)
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}
	if height == 1 {
		return w.heading(width)
	}

	plotHeight := height - 1
	now := time.Now()

	rx := w.values("rx", now, width)
	tx := w.values("tx", now, width)
	mirror := chart.Mirror{
		Up:     rx,
		Down:   tx,
		Range:  chart.Range{Max: chart.NiceMax(1024, rx, tx)},
		Glyphs: w.caps.Glyphs,
	}

	grid := mirror.Render(width, plotHeight)

	top := series.BytesPerSecond.Format(mirror.Top(), 0, series.Auto)
	grid.StampRight(0, top+" rx")
	grid.StampRight(plotHeight-1, top+" tx")

	lines := []string{w.heading(width)}
	lines = append(lines, w.paint(grid)...)

	return strings.Join(lines, "\n")
}

func (w *Widget) heading(width int) string {
	var (
		dim   = lipgloss.NewStyle().Faint(true)
		parts = []string{lipgloss.NewStyle().Bold(true).Render(safe.Text(w.iface))}
	)
	for _, attribute := range []string{"kind", "state", "ip", "address"} {
		if value, ok := w.store.Fact(string(w.key(attribute))); ok {
			parts = append(parts, dim.Render(value))
		}
	}

	if speed, ok := w.store.Last(w.key("speed")); ok {
		parts = append(parts, dim.Render("link "+series.BitsPerSecond.Format(speed.Value, 0, series.Auto)))
	}

	if total, ok := w.total("rx.total", "↓", w.Rx); ok {
		parts = append(parts, total)
	}

	if total, ok := w.total("tx.total", "↑", w.Tx); ok {
		parts = append(parts, total)
	}

	line := strings.Join(parts, "  ")

	return render.Left(line, width)
}

func (w *Widget) total(key, arrow string, colour color.Color) (string, bool) {
	point, ok := w.store.Last(w.key(key))
	if !ok {
		return "", false
	}

	value := arrow + " " + series.Bytes.Format(point.Value, 1, series.Auto)

	return render.Foreground(w.caps, colour).Render(value), true
}

func (w *Widget) values(what string, now time.Time, width int) []float64 {
	return widget.Resample(w.store.Window(w.key(what), w.Span), now, w.Span,
		chart.Slots(width, w.caps.Glyphs))
}

func (w *Widget) paint(grid chart.Grid) []string {
	var (
		dim  = lipgloss.NewStyle().Faint(true)
		up   = render.Foreground(w.caps, w.Rx)
		down = render.Foreground(w.caps, w.Tx)
	)

	return grid.Lines(func(owner chart.SeriesID) lipgloss.Style {
		switch owner {
		case chart.Label:
			return dim
		case 1:
			return down
		case chart.Empty:
			return lipgloss.NewStyle()
		default:
			return up
		}
	})
}
