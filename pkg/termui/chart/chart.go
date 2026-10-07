// Package chart renders numeric series as terminal-cell charts.
package chart

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Cell is one character of a rendered chart and its owning series.
type Cell struct {
	Rune   rune
	Series SeriesID
}

// SeriesID identifies the input series that owns a cell.
type SeriesID int16

const (
	// Empty marks a cell not owned by an input series.
	Empty SeriesID = -1

	// Label marks a cell owned by an annotation.
	Label SeriesID = -2
)

// Grid is a rectangular chart made of terminal cells.
type Grid [][]Cell

func newGrid(width, height int) Grid {
	grid := make(Grid, height)

	for y := range grid {
		grid[y] = make([]Cell, width)

		for x := range grid[y] {
			grid[y][x] = Cell{Rune: ' ', Series: Empty}
		}
	}

	return grid
}

// Plain returns the grid's glyphs without styling metadata.
func (g Grid) Plain() []string {
	lines := make([]string, len(g))

	for y, row := range g {
		runes := make([]rune, len(row))
		for x, cell := range row {
			runes[x] = cell.Rune
		}

		lines[y] = string(runes)
	}

	return lines
}

// Stamp writes an annotation if every destination cell is empty and in bounds.
func (g Grid) Stamp(row, from int, text string) bool {
	if row < 0 || row >= len(g) {
		return false
	}

	runes := []rune(text)
	for i := range runes {
		x := from + i
		if x < 0 || x >= len(g[row]) || g[row][x].Series != Empty {
			return false
		}
	}

	for i, r := range runes {
		g[row][from+i] = Cell{Rune: r, Series: Label}
	}

	return true
}

// StampRight places an annotation against the grid's right edge.
func (g Grid) StampRight(row int, text string) bool {
	if row < 0 || row >= len(g) {
		return false
	}

	return g.Stamp(row, len(g[row])-len([]rune(text)), text)
}

// Lines renders each row, styling adjacent cells with the same owner together.
func (g Grid) Lines(style func(SeriesID) lipgloss.Style) []string {
	lines := make([]string, 0, len(g))

	for _, row := range g {
		var (
			line    strings.Builder
			run     strings.Builder
			current = Empty
		)

		flush := func() {
			if run.Len() == 0 {
				return
			}

			line.WriteString(style(current).Render(run.String()))
			run.Reset()
		}

		for _, cell := range row {
			if cell.Series != current {
				flush()
				current = cell.Series
			}
			run.WriteRune(cell.Rune)
		}
		flush()

		lines = append(lines, line.String())
	}

	return lines
}

// Render returns the styled grid as newline-separated rows.
func (g Grid) Render(style func(SeriesID) lipgloss.Style) string {
	return strings.Join(g.Lines(style), "\n")
}

// Range is a chart's vertical extent. Max values at or below Min enable the
// automatic range.
type Range struct {
	Min, Max float64
}

// NiceMax returns a readable automatic ceiling rounded to a 1/2/5 step.
func NiceMax(floor float64, values ...[]float64) float64 {
	peak := 0.0
	if finite(floor) {
		peak = max(0, floor)
	}
	for _, set := range values {
		for _, value := range set {
			if finite(value) {
				peak = max(peak, value)
			}
		}
	}

	// Reserve space for the upper axis label.
	wanted := peak * 1.1
	if wanted <= 0 {
		return 1
	}

	power := math.Pow(10, math.Floor(math.Log10(wanted)))
	normalized := wanted / power
	for _, step := range []float64{1, 2, 5, 10} {
		if normalized <= step {
			return step * power
		}
	}

	return 10 * power
}

func (r Range) resolve(series [][]float64) (float64, float64) {
	if finite(r.Min) && finite(r.Max) && r.Max > r.Min {
		return r.Min, r.Max
	}

	low, high := 0.0, 0.0

	for _, values := range series {
		for _, v := range values {
			if !finite(v) {
				continue
			}
			high = max(high, v)
			low = min(low, v)
		}
	}

	if high <= low {
		high = low + 1
	}

	// Keep peaks below the top edge.
	return low, high * 1.05
}

// sample right-aligns values into slots, averaging when there are more values
// than slots. A monitor scrolls left, so the newest reading owns the last slot.
func sample(values []float64, slots int) []float64 {
	if slots <= 0 {
		return nil
	}

	out := make([]float64, slots)
	for i := range out {
		out[i] = nan
	}

	if len(values) == 0 {
		return out
	}

	if len(values) <= slots {
		copy(out[slots-len(values):], values)

		return out
	}

	perSlot := float64(len(values)) / float64(slots)

	for i := range slots {
		var (
			from  = int(float64(i) * perSlot)
			to    = min(len(values), int(float64(i+1)*perSlot))
			sum   float64
			count int
		)

		for _, v := range values[from:max(to, from+1)] {
			if !finite(v) {
				continue
			}
			sum += v
			count++
		}

		if count > 0 {
			out[i] = sum / float64(count)
		}
	}

	return out
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
