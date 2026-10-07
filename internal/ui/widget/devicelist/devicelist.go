// Package devicelist draws a device rail with thumbnail graphs and readings.
package devicelist

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Entry describes one device rail item.
type Entry struct {
	Title  string
	Route  string
	Key    series.Key
	Unit   series.Unit
	Colour color.Color
	Max    float64
}

// rows includes a title, two plot rows, and a separator.
const rows = 4

// Widget renders and navigates a device rail.
type Widget struct {
	reactea.BasicComponent

	store   *store.Store
	caps    render.Caps
	theme   *theme.Theme
	entries []Entry
	route   string

	step   int
	offset int
	// firstRow excludes the optional overflow indicator from click coordinates.
	firstRow int
	visible  int
}

// New constructs a device rail.
func New(s *store.Store, caps render.Caps, t *theme.Theme, entries ...Entry) *Widget {
	return &Widget{store: s, caps: caps, theme: t, entries: entries, step: rows}
}

// Entries returns the current rail entries.
func (w *Widget) Entries() []Entry { return w.entries }

// SetEntries replaces the rail entries.
func (w *Widget) SetEntries(entries []Entry) { w.entries = entries }

func (w *Widget) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok || click.Button != tea.MouseLeft {
		return nil
	}

	if _, _, inside := reactea.Mouse(ctx, msg); !inside {
		return nil
	}

	row := click.Y - w.firstRow
	if row < 0 || row >= w.visible*max(1, w.step) {
		return nil
	}

	index := w.offset + row/max(1, w.step)
	if index >= len(w.entries) {
		return nil
	}

	return ctx.SetRoute(w.entries[index].Route)
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	w.route = ctx.Route()

	w.step = rows
	selected := 0
	for index, entry := range w.entries {
		if entry.Route == w.route {
			selected = index
			break
		}
	}
	w.offset, w.visible = entryWindow(len(w.entries), height, selected, w.offset)

	var lines []string
	w.firstRow = 0
	if w.offset > 0 && w.visible*rows+1 <= height {
		lines = append(lines, w.indicator(width, true, w.offset))
		w.firstRow = 1
	}

	end := min(len(w.entries), w.offset+w.visible)
	for _, entry := range w.entries[w.offset:end] {
		lines = append(lines, w.entry(entry, width)...)
	}

	below := len(w.entries) - end
	if below > 0 && len(lines) < height {
		for len(lines) < height-1 {
			lines = append(lines, strings.Repeat(" ", width))
		}
		lines = append(lines, w.indicator(width, false, below))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines[:height], "\n")
}

func entryWindow(total, height, selected, preferred int) (offset, visible int) {
	if total == 0 || height <= 0 {
		return 0, 0
	}

	visible = min(total, max(1, height/rows))
	for {
		offset = min(max(0, total-visible), max(0, preferred))
		if selected < offset {
			offset = selected
		}
		if selected >= offset+visible {
			offset = selected - visible + 1
		}

		indicators := 0
		if offset > 0 {
			indicators++
		}
		if offset+visible < total {
			indicators++
		}
		if visible == 1 || visible*rows+indicators <= height {
			return offset, visible
		}

		visible--
	}
}

func (w *Widget) indicator(width int, above bool, hidden int) string {
	arrow := "↓"
	if above {
		arrow = "↑"
	}
	if w.caps.Glyphs == render.ASCII {
		arrow = "v"
		if above {
			arrow = "^"
		}
	}

	return render.Left(w.theme.Style(theme.Dim).Render(
		fmt.Sprintf("  %s %d more", arrow, hidden)), width)
}

func (w *Widget) entry(entry Entry, width int) []string {
	var (
		selected = w.route == entry.Route
		title    = w.theme.Style(theme.Text).Bold(selected)
		reading  = render.Foreground(w.caps, entry.Colour)
		marker   = "  "
	)

	if selected {
		marker = "▎ "
		title = w.theme.Style(theme.Heading)
	}

	value := "—"
	if point, ok := w.store.Last(entry.Key); ok {
		value = entry.Unit.Format(point.Value, 0, series.Auto)
	}

	points := w.store.Window(entry.Key, time.Minute)
	thumbWidth := max(1, width-len(marker)-1)

	thumb := chart.Line{
		Series: [][]float64{widget.Resample(points, time.Now(), time.Minute, chart.Slots(thumbWidth, w.caps.Glyphs))},
		Range:  chart.Range{Max: entry.Max},
		Glyphs: w.caps.Glyphs,
		Fill:   true,
	}.Render(thumbWidth, 2)

	lines := []string{render.Sides(marker+title.Render(entry.Title), reading.Render(value)+" ", width)}

	for _, row := range thumb.Lines(func(owner chart.SeriesID) lipgloss.Style {
		if owner >= 0 {
			return reading
		}
		return lipgloss.NewStyle()
	}) {
		lines = append(lines, render.Left("  "+row, width))
	}

	lines = append(lines, "  "+w.theme.Style(theme.Border).Render(strings.Repeat("┄", max(0, width-2))))

	return lines
}
