package chart_test

import (
	"math"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

func TestMirrorSeparatesSeriesAndUsesSharedScale(t *testing.T) {
	t.Parallel()

	mirror := chart.Mirror{
		Up: []float64{0, 100}, Down: []float64{0, 50},
		Range: chart.Range{Min: 0, Max: 100}, Glyphs: chart.Braille,
	}
	grid := mirror.Render(2, 4)
	if len(grid) != 4 || mirror.Top() != 100 {
		t.Fatalf("mirror size=%d top=%v", len(grid), mirror.Top())
	}

	var owners [2]bool
	for _, row := range grid {
		for _, cell := range row {
			if cell.Series == 0 || cell.Series == 1 {
				owners[cell.Series] = true
			}
		}
	}
	if !owners[0] || !owners[1] {
		t.Fatalf("series ownership = %v", owners)
	}
	if got := (chart.Mirror{}).Render(0, 0); len(got) != 0 {
		t.Fatalf("zero mirror has %d rows", len(got))
	}
}

func TestMirrorAutoRangeDoesNotKeepAnInvalidMinimum(t *testing.T) {
	t.Parallel()

	mirror := chart.Mirror{
		Up: []float64{2}, Down: []float64{8},
		Range: chart.Range{Min: 10}, Glyphs: chart.Block,
	}
	grid := mirror.Render(1, 4)

	if mirror.Top() <= 8 {
		t.Fatalf("automatic top = %v, want headroom above 8", mirror.Top())
	}
	if grid[1][0].Rune == ' ' {
		t.Fatalf("the smaller series disappeared under an invalid retained minimum:\n%s",
			strings.Join(grid.Plain(), "\n"))
	}
}

func TestBlockMirrorGrowsDownFromTheMiddle(t *testing.T) {
	t.Parallel()

	grid := chart.Mirror{
		Down:  []float64{6.25, 25, 75, 100},
		Range: chart.Range{Min: 0, Max: 100}, Glyphs: chart.Block,
	}.Render(4, 4)

	want := "    \n    \n▔▀██\n  ▀█"
	if got := strings.Join(grid.Plain(), "\n"); got != want {
		t.Errorf("block mirror:\ngot\n%s\nwant\n%s", got, want)
	}
}

func TestGaugeClampsAndHandlesNaN(t *testing.T) {
	t.Parallel()

	for _, fraction := range []float64{-1, math.NaN()} {
		filled, rest := chart.Gauge(fraction, 4, chart.ASCII)
		if filled != "" || rest != "...." {
			t.Fatalf("Gauge(%v) = %q %q", fraction, filled, rest)
		}
	}
	filled, rest := chart.Gauge(2, 4, chart.Block)
	if filled != "████" || rest != "" {
		t.Fatalf("clamped gauge = %q %q", filled, rest)
	}
	filled, rest = chart.Gauge(.5, 3, chart.Block)
	if strings.Count(filled+rest, "") == 0 || len([]rune(filled+rest)) != 3 {
		t.Fatalf("partial gauge width = %q %q", filled, rest)
	}
	if filled, rest := chart.Gauge(.5, 0, chart.Block); filled != "" || rest != "" {
		t.Fatal("zero-width gauge is not empty")
	}
}

func TestSlotsAndExtent(t *testing.T) {
	t.Parallel()

	if chart.Slots(5, chart.Braille) != 10 || chart.Slots(5, chart.Block) != 5 {
		t.Fatal("slot resolution is wrong")
	}
	if chart.Slots(-1, chart.Braille) != 0 {
		t.Fatal("negative width produced slots")
	}
	low, high := (chart.Line{Series: [][]float64{{2, 9}}, Range: chart.Range{Min: 1, Max: 10}}).Extent()
	if low != 1 || high != 10 {
		t.Fatalf("extent = %v, %v", low, high)
	}
}
