package chart

import (
	"math"
)

var nan = math.NaN()

// Line renders one or more series as a line or filled-area chart.
type Line struct {
	// Series contains one slice of values per visual series.
	Series [][]float64
	// Range controls vertical scaling; a zero range enables automatic scaling.
	Range Range
	// Glyphs selects the chart's character set.
	Glyphs Glyphs
	// Fill draws the area below each sample when true.
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
	grid := newGrid(width, height)

	low, high := l.Range.resolve(l.Series)

	switch l.Glyphs {
	case Braille:
		l.braille(grid, width, height, low, high)
	case Block:
		l.blocks(grid, width, height, low, high, blockRunes)
	default:
		l.blocks(grid, width, height, low, high, asciiRunes)
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

func (l Line) braille(grid Grid, width, height int, low, high float64) {
	var (
		dotsX = width * 2
		dotsY = height * 4
		bits  = make([]rune, width*height)
		owner = make([]SeriesID, width*height)
	)

	for i := range owner {
		owner[i] = Empty
	}

	// Later series take precedence in shared cells.
	for s, values := range l.Series {
		points := sample(values, dotsX)

		for x, v := range points {
			if !finite(v) {
				continue
			}

			covered := fill(v, low, high, dotsY)
			if covered == 0 && l.Fill {
				continue
			}

			top := max(0, dotsY-max(covered, 1))

			to := dotsY - 1
			if !l.Fill {
				to = top
			}

			for y := top; y <= to; y++ {
				var (
					cell = (y/4)*width + x/2
					bit  = brailleBits[y%4][x%2]
				)

				bits[cell] |= bit
				owner[cell] = SeriesID(s)
			}
		}
	}

	for i, b := range bits {
		if b == 0 {
			continue
		}

		grid[i/width][i%width] = Cell{Rune: 0x2800 + b, Series: owner[i]}
	}
}

var (
	blockRunes = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	asciiRunes = []rune{' ', '.', '.', '-', '-', '=', '=', '#', '#'}
)

func (l Line) blocks(grid Grid, width, height int, low, high float64, runes []rune) {
	eighths := height * 8

	for s, values := range l.Series {
		points := sample(values, width)

		for x, v := range points {
			if !finite(v) {
				continue
			}

			filled := fill(v, low, high, eighths)
			if !l.Fill {
				if filled == 0 {
					grid[height-1][x] = Cell{Rune: runes[1], Series: SeriesID(s)}

					continue
				}

				y := min((eighths-filled)/8, height-1)
				remaining := filled - (height-1-y)*8
				grid[y][x] = Cell{Rune: runes[min(remaining, 8)], Series: SeriesID(s)}

				continue
			}

			for y := range height {
				var (
					fromBottom = height - 1 - y
					remaining  = filled - fromBottom*8
				)

				switch {
				case remaining <= 0:
					continue
				case remaining >= 8:
					grid[y][x] = Cell{Rune: runes[8], Series: SeriesID(s)}
				default:
					grid[y][x] = Cell{Rune: runes[remaining], Series: SeriesID(s)}
				}
			}
		}
	}
}

// fill is how many of steps sub-units v covers, 0 to steps inclusive, so a
// reading at the top of the range fills the column completely.
func fill(v, low, high float64, steps int) int {
	if high <= low {
		return 0
	}

	ratio := (v - low) / (high - low)

	return min(max(int(math.Round(ratio*float64(steps))), 0), steps)
}
