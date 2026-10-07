package chart

// Mirror draws Up above and Down below a shared baseline and scale.
type Mirror struct {
	// Up and Down contain samples drawn above and below the baseline.
	Up, Down []float64
	// Range controls the shared scale; a zero range enables automatic scaling.
	Range Range
	// Glyphs selects the chart's character set.
	Glyphs Glyphs
}

// Render draws both halves into a grid of width by height cells.
func (m Mirror) Render(width, height int) Grid {
	if width <= 0 || height <= 0 {
		return Grid{}
	}
	grid := newGrid(width, height)

	upper := height / 2
	lower := height - upper

	shared := m.Range
	if shared.Max <= shared.Min {
		shared.Min, shared.Max = Range{}.resolve([][]float64{m.Up, m.Down})
	}

	if upper > 0 {
		up := Line{Series: [][]float64{m.Up}, Range: shared, Glyphs: m.Glyphs, Fill: true}.Render(width, upper)
		copy(grid[:upper], up)
	}

	if lower > 0 {
		down := Line{Series: [][]float64{m.Down}, Range: shared, Glyphs: m.Glyphs, Fill: true}.Render(width, lower)

		for y := range lower {
			grid[upper+y] = flipRow(down[lower-1-y], m.Glyphs, 1)
		}
	}

	return grid
}

// Top returns the maximum used to scale both halves.
func (m Mirror) Top() float64 {
	if m.Range.Max > m.Range.Min {
		return m.Range.Max
	}

	_, top := Range{}.resolve([][]float64{m.Up, m.Down})

	return top
}

func flipRow(row []Cell, glyphs Glyphs, series int) []Cell {
	flipped := make([]Cell, len(row))

	for x, cell := range row {
		flipped[x] = cell

		switch {
		case glyphs == Braille && cell.Rune >= 0x2800 && cell.Rune <= 0x28ff:
			flipped[x].Rune = 0x2800 + brailleFlip[cell.Rune-0x2800]
		case glyphs == Block:
			flipped[x].Rune = blockFlip(cell.Rune)
		}

		if cell.Series != Empty && cell.Series != Label {
			flipped[x].Series = SeriesID(series)
		}
	}

	return flipped
}

func blockFlip(r rune) rune {
	switch r {
	case '▁', '▂':
		return '▔'
	case '▃', '▄', '▅':
		return '▀'
	case '▆', '▇':
		return '█'
	default:
		return r
	}
}

var brailleFlip = buildBrailleFlip()

func buildBrailleFlip() [256]rune {
	var table [256]rune

	for bits := range 256 {
		var flipped rune

		for row := range 4 {
			for column := range 2 {
				if rune(bits)&brailleBits[row][column] != 0 {
					flipped |= brailleBits[3-row][column]
				}
			}
		}

		table[bits] = flipped
	}

	return table
}
