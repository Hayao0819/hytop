package chart_test

import (
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

func TestCellRemainsCompact(t *testing.T) {
	t.Parallel()

	if size := unsafe.Sizeof(chart.Cell{}); size != 8 {
		t.Fatalf("Cell occupies %d bytes, want 8", size)
	}
}

func ramp(n int) []float64 {
	values := make([]float64, n)
	for i := range values {
		values[i] = float64(i) * 100 / float64(n-1)
	}

	return values
}

func show(g chart.Grid) string { return strings.Join(g.Plain(), "\n") }

func TestBrailleFillRises(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{ramp(16)},
		Range:  chart.Range{Min: 0, Max: 100},
		Glyphs: chart.Braille,
		Fill:   true,
	}.Render(8, 4)

	got := show(g)

	want := strings.Join([]string{
		"      ⣠⣾",
		"    ⣠⣾⣿⣿",
		"  ⢀⣴⣿⣿⣿⣿",
		"⢀⣴⣿⣿⣿⣿⣿⣿",
	}, "\n")

	if got != want {
		t.Errorf("braille fill:\ngot\n%s\nwant\n%s", got, want)
	}
}

func TestBlockFallbackKeepsTheShape(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{ramp(8)},
		Range:  chart.Range{Min: 0, Max: 100},
		Glyphs: chart.Block,
		Fill:   true,
	}.Render(8, 3)

	lines := g.Plain()

	want := []string{"     ▁▅█", "   ▂▆███", " ▃▇█████"}
	if !slices.Equal(lines, want) {
		t.Errorf("block fallback:\ngot\n%s\nwant\n%s", show(g), strings.Join(want, "\n"))
	}
}

func TestASCIIUsesNoWideRunes(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{ramp(8)},
		Range:  chart.Range{Min: 0, Max: 100},
		Glyphs: chart.ASCII,
		Fill:   true,
	}.Render(8, 3)

	for _, line := range g.Plain() {
		for _, r := range line {
			if r > 127 {
				t.Fatalf("non-ASCII rune %q in %q", r, line)
			}
		}
	}

	want := "     .=#\n   .=###\n -######"
	if got := show(g); got != want {
		t.Errorf("ASCII fallback:\ngot\n%s\nwant\n%s", got, want)
	}
}

func TestTheNewestReadingOwnsTheRightEdge(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{{100, 100}},
		Range:  chart.Range{Min: 0, Max: 100},
		Glyphs: chart.Block,
		Fill:   true,
	}.Render(6, 1)

	line := g.Plain()[0]

	if !strings.HasSuffix(line, "██") || strings.TrimLeft(line, " ") != "██" {
		t.Errorf("not right-aligned: %q", line)
	}
}

func TestOverlaidSeriesKeepTheirOwnColour(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{
			{10, 10, 10, 10},
			{90, 90, 90, 90},
		},
		Range:  chart.Range{Min: 0, Max: 100},
		Glyphs: chart.Block,
		Fill:   false,
	}.Render(4, 4)

	var seen [2]bool

	for _, row := range g {
		for _, cell := range row {
			if cell.Series >= 0 {
				seen[cell.Series] = true
			}
		}
	}

	if !seen[0] || !seen[1] {
		t.Errorf("a series was lost:\n%s", show(g))
	}
}

func TestUnfilledFallbackDrawsOneCellAtRowBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		glyphs chart.Glyphs
		want   string
	}{
		{"block", chart.Block, "      ▄█\n    ▄█  \n  ▄█    \n▄█      "},
		{"ASCII", chart.ASCII, "      -#\n    -#  \n  -#    \n-#      "},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			grid := chart.Line{
				Series: [][]float64{{12.5, 25, 37.5, 50, 62.5, 75, 87.5, 100}},
				Range:  chart.Range{Min: 0, Max: 100},
				Glyphs: test.glyphs,
				Fill:   false,
			}.Render(8, 4)

			if got := show(grid); got != test.want {
				t.Errorf("unfilled fallback:\ngot\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

func TestUnfilledLineKeepsTheMinimumVisible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		glyphs chart.Glyphs
		want   rune
	}{
		{chart.Braille, '⢀'},
		{chart.Block, '▁'},
		{chart.ASCII, '.'},
	}

	for _, test := range tests {
		grid := chart.Line{
			Series: [][]float64{{0}},
			Range:  chart.Range{Min: 0, Max: 100},
			Glyphs: test.glyphs,
			Fill:   false,
		}.Render(1, 2)

		if got := grid[1][0].Rune; got != test.want {
			t.Errorf("%v minimum rune = %q, want %q:\n%s", test.glyphs, got, test.want, show(grid))
		}
	}
}

func TestAutoRangeLeavesHeadroom(t *testing.T) {
	t.Parallel()

	g := chart.Line{
		Series: [][]float64{{1, 2, 3}},
		Glyphs: chart.Block,
		Fill:   true,
	}.Render(3, 4)

	if strings.Contains(g.Plain()[0], "█") {
		t.Errorf("the peak filled the top row with an auto range:\n%s", show(g))
	}
}

func TestEmptyInputDrawsBlank(t *testing.T) {
	t.Parallel()

	for _, l := range []chart.Line{
		{Glyphs: chart.Braille},
		{Series: [][]float64{{}}, Glyphs: chart.Braille},
	} {
		if got := strings.TrimSpace(show(l.Render(4, 2))); got != "" {
			t.Errorf("drew %q for empty input", got)
		}
	}

	if got := l0().Render(0, 0); len(got) != 0 {
		t.Errorf("a zero-sized chart returned %d rows", len(got))
	}
	if got := l0().Render(-1, -1); len(got) != 0 {
		t.Errorf("a negative-sized chart returned %d rows", len(got))
	}
}

func TestAutoRangeIgnoresMissingReadings(t *testing.T) {
	t.Parallel()

	line := chart.Line{Series: [][]float64{{math.NaN(), 5}}, Glyphs: chart.Block}
	low, high := line.Extent()
	if math.IsNaN(low) || math.IsNaN(high) || low != 0 || high <= 5 {
		t.Fatalf("Extent = %v, %v, want a finite range above 5", low, high)
	}
	if got := strings.TrimSpace(show(line.Render(2, 2))); got == "" {
		t.Fatal("a finite reading beside a missing reading was not drawn")
	}
}

func TestAutoRangeIgnoresInfiniteReadings(t *testing.T) {
	t.Parallel()

	line := chart.Line{Series: [][]float64{{math.Inf(-1), 5, math.Inf(1)}}, Glyphs: chart.Block}
	low, high := line.Extent()
	if math.IsInf(low, 0) || math.IsInf(high, 0) || low != 0 || high <= 5 {
		t.Fatalf("Extent = %v, %v, want a finite range above 5", low, high)
	}
	if got := strings.TrimSpace(show(line.Render(3, 2))); got == "" {
		t.Fatal("the finite reading beside infinite readings was not drawn")
	}
}

func TestStampIsAtomicAndRightAligned(t *testing.T) {
	t.Parallel()

	grid := chart.Line{Series: [][]float64{{50}}, Range: chart.Range{Max: 100}, Glyphs: chart.Block}.
		Render(6, 2)
	before := show(grid)

	if grid.Stamp(1, 4, "long") {
		t.Fatal("out-of-bounds annotation was accepted")
	}
	if show(grid) != before {
		t.Fatal("failed annotation partially changed the grid")
	}
	if !grid.StampRight(0, "ok") {
		t.Fatal("right annotation was not placed")
	}
	if !strings.HasSuffix(grid.Plain()[0], "ok") {
		t.Fatalf("annotation was not right aligned: %q", grid.Plain()[0])
	}
}

func TestGridRenderStylesRunsByOwner(t *testing.T) {
	t.Parallel()

	grid := chart.Line{Series: [][]float64{{100}}, Range: chart.Range{Max: 100}, Glyphs: chart.Block}.
		Render(1, 1)
	red := color.RGBA{R: 255, A: 255}
	drawn := grid.Render(func(owner chart.SeriesID) lipgloss.Style {
		if owner == 0 {
			return lipgloss.NewStyle().Foreground(red)
		}
		return lipgloss.NewStyle()
	})

	if !strings.Contains(drawn, "\x1b[") || lipgloss.Width(drawn) != 1 {
		t.Fatalf("styled render = %q", drawn)
	}
}

func l0() chart.Line {
	return chart.Line{Series: [][]float64{{1}}, Glyphs: chart.Braille}
}
