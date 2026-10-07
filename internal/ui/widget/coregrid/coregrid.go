// Package coregrid draws one small plot per logical processor.
package coregrid

import (
	"image/color"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/widget"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

type Widget struct {
	reactea.BasicComponent

	store  *store.Store
	caps   render.Caps
	colour color.Color

	Span   time.Duration
	Border color.Color
	Keys   func() []series.Key
}

func New(s *store.Store, caps render.Caps, colour color.Color, keys func() []series.Key) *Widget {
	return &Widget{
		store: s, caps: caps, colour: colour,
		Span: time.Minute, Border: lipgloss.Color("240"), Keys: keys,
	}
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()

	keys := w.Keys()
	if width <= 0 || height <= 0 || len(keys) == 0 {
		return ""
	}

	shape, ok := fit(len(keys), width, height)
	if !ok {
		return lipgloss.NewStyle().Faint(true).Render("too small for a plot per core")
	}

	var lines []string

	for row := range shape.rows {
		cells := make([][]string, 0, shape.columns)

		for column := range shape.columns {
			index := row*shape.columns + column

			cellW := shape.width(column)
			cellH := shape.height(row)

			if index >= len(keys) {
				cells = append(cells, blank(cellW, cellH))

				continue
			}

			cells = append(cells, w.cell(keys[index], cellW, cellH, shape.bordered))
		}

		lines = append(lines, side(cells, shape.height(row))...)
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines[:height], "\n")
}

// shape is a chosen arrangement, and how the cells that do not divide evenly
// share the leftover rows and columns.
type shape struct {
	columns, rows        int
	cellW, cellH         int
	extraCols, extraRows int
	bordered             bool
}

func (s shape) width(column int) int {
	if column < s.extraCols {
		return s.cellW + 1
	}

	return s.cellW
}

func (s shape) height(row int) int {
	if row < s.extraRows {
		return s.cellH + 1
	}

	return s.cellH
}

// Borders are removed before plots when the available space shrinks.
const (
	borderedW = 12
	borderedH = 5

	bareW = 9
	bareH = 3

	// A terminal cell is about twice as tall as it is wide, so a plot looks
	// square when it is twice as many columns as rows.
	cellAspect = 2
)

// fit balances square cells against unused grid positions.
func fit(count, width, height int) (shape, bool) {
	if best, ok := arrange(count, width, height, borderedW, borderedH); ok {
		best.bordered = true

		return best, true
	}

	return arrange(count, width, height, bareW, bareH)
}

func arrange(count, width, height, minW, minH int) (shape, bool) {
	var (
		best  shape
		score = math.Inf(1)
		found bool
	)

	for columns := 1; columns <= count; columns++ {
		rows := (count + columns - 1) / columns

		cellW := width / columns
		cellH := height / rows

		if cellW < minW || cellH < minH {
			continue
		}

		// Distance from a square cell, plus a penalty for each place left empty.
		candidate := math.Abs(float64(cellW)-cellAspect*float64(cellH)) +
			3*float64(columns*rows-count)

		if candidate < score {
			best = shape{
				columns:   columns,
				rows:      rows,
				cellW:     cellW,
				cellH:     cellH,
				extraCols: width % columns,
				extraRows: height % rows,
			}
			score, found = candidate, true
		}
	}

	return best, found
}

func (w *Widget) cell(key series.Key, width, height int, bordered bool) []string {
	// One column of air, or two for a border, keeps neighbouring fills from
	// reading as one plot.
	inner := width - 1
	if bordered {
		inner = width - 2
	}

	points := w.store.Window(key, w.Span)

	plotHeight := max(1, height-1)
	if bordered {
		plotHeight = max(1, height-3)
	}

	grid := chart.Line{
		Series: [][]float64{widget.Resample(points, time.Now(), w.Span, chart.Slots(inner, w.caps.Glyphs))},
		Range:  chart.Range{Max: 100},
		Glyphs: w.caps.Glyphs,
		Fill:   true,
	}.Render(inner, plotHeight)

	plot := render.Foreground(w.caps, w.colour)

	body := make([]string, 0, height)
	body = append(body, heading(key, points, inner))

	body = append(body, grid.Lines(func(owner chart.SeriesID) lipgloss.Style {
		if owner >= 0 {
			return plot
		}
		return lipgloss.NewStyle()
	})...)

	if !bordered {
		for i, line := range body {
			body[i] = line + " "
		}

		return body
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width).Height(height).
		MaxWidth(width).MaxHeight(height)
	if w.caps.Colors != render.Mono && w.Border != nil {
		frame = frame.BorderForeground(w.Border)
	}

	framed := frame.Render(strings.Join(body, "\n"))

	return strings.Split(framed, "\n")
}

// heading is the core's number on the left and its reading on the right, which
// is where the eye looks for each.
func heading(key series.Key, points []metric.Point, width int) string {
	var (
		dim   = lipgloss.NewStyle().Faint(true)
		name  = shortName(key)
		value = current(points)
	)

	return dim.Render(render.Sides(name, value, width))
}

func current(points []metric.Point) string {
	if len(points) == 0 {
		return "—"
	}

	v := points[len(points)-1].Value
	if math.IsNaN(v) {
		return "—"
	}

	return series.Percent.Format(v, 0, series.Auto)
}

func shortName(key series.Key) string {
	segments := key.Segments()
	if len(segments) >= 3 {
		return segments[2]
	}

	return string(key)
}

func blank(width, height int) []string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}

	return lines
}

func side(cells [][]string, height int) []string {
	lines := make([]string, height)

	for row := range height {
		var b strings.Builder

		for _, cell := range cells {
			if row < len(cell) {
				b.WriteString(cell[row])
			}
		}

		lines[row] = b.String()
	}

	return lines
}
