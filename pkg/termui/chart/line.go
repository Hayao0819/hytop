package chart

import (
	"image"
	"math"
)

var nan = math.NaN()

// Line renders one or more series as a line or filled-area chart.
type Line struct {
	// Series contains one slice per series; non-finite values break the line.
	Series [][]float64
	// Range controls vertical scaling; a zero range enables automatic scaling.
	Range Range
	// Glyphs selects the chart's character set.
	Glyphs Glyphs
	// Fill adds the area below the line.
	Fill bool
}

// Slots returns the number of samples that fill width at the selected glyph
// resolution.
func Slots(width int, glyphs Glyphs) int {
	if width <= 0 {
		return 0
	}

	if glyphs == Braille {
		return width * 2
	}

	return width
}

// Extent returns the range used to scale the chart.
func (l Line) Extent() (float64, float64) { return l.Range.resolve(l.Series) }

// Render draws the line into a grid of width by height cells.
func (l Line) Render(width, height int) Grid {
	if width <= 0 || height <= 0 || len(l.Series) == 0 {
		return Grid{}
	}
	var (
		grid       = newGrid(width, height)
		resolution = image.Pt(Slots(width, l.Glyphs), height*8)
		runes      = asciiRunes
	)
	switch l.Glyphs {
	case Braille:
		resolution.Y = height * 4
	case Block:
		runes = blockRunes
	}
	low, high := l.Range.resolve(l.Series)

	// Later series take precedence in shared cells.
	for s, values := range l.Series {
		path := projectLine(values, resolution, low, high)
		for x, column := range path {
			if column.bottom == 0 {
				continue
			}
			if l.Fill {
				column.bottom = resolution.Y
			}
			if l.Glyphs == Braille {
				drawBrailleColumn(grid, x, column, SeriesID(s))
			} else {
				drawBlockColumn(grid, x, column, SeriesID(s), runes)
			}
		}
	}

	return grid
}

// Braille dot bits, by row then column.
var brailleBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

func drawBrailleColumn(grid Grid, x int, column columnSpan, owner SeriesID) {
	for y := column.top; y < column.bottom; y++ {
		cell := &grid[y/4][x/2]
		if cell.Series == Empty {
			cell.Rune = 0x2800
		}
		cell.Rune |= brailleBits[y%4][x%2]
		cell.Series = owner
	}
}

var (
	blockRunes = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	asciiRunes = []rune{' ', '.', '.', '-', '-', '=', '=', '#', '#'}
)

func drawBlockColumn(grid Grid, x int, column columnSpan, owner SeriesID, runes []rune) {
	for y := column.top / 8; y <= (column.bottom-1)/8; y++ {
		covered := min((y+1)*8-column.top, 8)
		grid[y][x] = Cell{Rune: runes[covered], Series: owner}
	}
}

// fill is how many of steps sub-units v covers, 0 to steps inclusive, so a
// reading at the top of the range fills the column completely.
func fill(v, low, high float64, steps int) int {
	if high <= low {
		return 0
	}

	ratio := min(max((v-low)/(high-low), 0), 1)

	return int(math.Round(ratio * float64(steps)))
}
