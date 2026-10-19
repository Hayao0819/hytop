package chart_test

import (
	"fmt"
	"image"
	"math"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/pkg/termui/chart"
	"github.com/Hayao0819/hytop/pkg/termui/timeseries"
)

func TestLineConnectsReadings(t *testing.T) {
	t.Parallel()

	shapes := [][]float64{
		{0, 0, 100, 100},
		{100, 100, 0, 0},
		{0, 100, 0, 100, 0, 100},
		{50, 50, 50, 50},
		{0, 0, 0, 0},
		{-100, 200, -100, 200},
	}
	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		for _, fill := range []bool{false, true} {
			for _, size := range [][2]int{{1, 1}, {1, 12}, {2, 1}, {2, 16}, {12, 6}, {47, 23}} {
				for i, values := range shapes {
					t.Run(fmt.Sprintf("%s/fill=%t/%dx%d/shape=%d", glyphs, fill, size[0], size[1], i), func(t *testing.T) {
						grid := chart.Line{
							Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs, Fill: fill,
						}.Render(size[0], size[1])
						if got := components(grid, glyphs); got != 1 {
							t.Fatalf("line has %d connected components, want 1:\n%s", got, show(grid))
						}
					})
				}
			}
		}
	}
}

func TestLineKeepsMissingIntervalsEmpty(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		for _, fill := range []bool{false, true} {
			values := make([]float64, chart.Slots(14, glyphs))
			for i := range values {
				x := i * 14 / len(values)
				switch {
				case x < 2 || (x >= 6 && x < 10):
					values[i] = math.NaN()
				case x == 3:
					values[i] = math.Inf(1)
				case x >= 4 && x < 12:
					values[i] = 100
				}
			}
			grid := chart.Line{
				Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs, Fill: fill,
			}.Render(14, 8)
			if got := components(grid, glyphs); got != 3 {
				t.Fatalf("%s fill=%t: got %d components, want 3:\n%s", glyphs, fill, got, show(grid))
			}
			for _, row := range grid {
				for _, x := range []int{0, 1, 3, 6, 7, 8, 9} {
					if row[x].Series != chart.Empty {
						t.Fatalf("%s fill=%t: missing column %d was drawn:\n%s", glyphs, fill, x, show(grid))
					}
				}
			}
		}
	}
}

func TestLineFillPreservesTheStroke(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		line := chart.Line{
			Series: [][]float64{{0, 100, 0, 50, 0, 100}}, Range: chart.Range{Max: 100}, Glyphs: glyphs,
		}
		stroke := dots(line.Render(12, 8), glyphs)
		line.Fill = true
		filled := dots(line.Render(12, 8), glyphs)
		for point := range stroke {
			if !filled[point] {
				t.Fatalf("%s: fill removed stroke point %v", glyphs, point)
			}
		}
	}
}

func TestLinePassesThroughReadings(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		values := make([]float64, chart.Slots(8, glyphs))
		for i := range values {
			values[i] = float64(i%5) * 25
		}
		line := chart.Line{Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs}
		points := dots(line.Render(8, 8), glyphs)
		rows := 8 * 8
		if glyphs == chart.Braille {
			rows = 8 * 4
		}
		for x, value := range values {
			y := rows - max(int(math.Round(value*float64(rows)/100)), 1)
			if glyphs != chart.Braille {
				y /= 8
			}
			if !points[image.Pt(x, y)] {
				t.Fatalf("%s: line missed reading %d (%g)", glyphs, x, value)
			}
		}
	}
}

func TestLineOverlappingSeriesUseTheLastColour(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		for _, fill := range []bool{false, true} {
			line := chart.Line{
				Series: [][]float64{{0, 100, 0}, {0, 100, 0}},
				Range:  chart.Range{Max: 100}, Glyphs: glyphs, Fill: fill,
			}
			for _, row := range line.Render(12, 8) {
				for _, cell := range row {
					if cell.Series >= 0 && cell.Series != 1 {
						t.Fatalf("%s fill=%t: shared cell belongs to series %d, want 1", glyphs, fill, cell.Series)
					}
				}
			}
		}
	}
}

func TestUnfilledLineDoesNotFillBelowTheStroke(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		values := make([]float64, chart.Slots(8, glyphs))
		for i := range values {
			values[i] = 75
		}
		grid := chart.Line{
			Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs,
		}.Render(8, 8)
		if got := len(dots(grid, glyphs)); got != len(values) {
			t.Fatalf("%s: horizontal stroke occupies %d points, want %d:\n%s", glyphs, got, len(values), show(grid))
		}
	}
}

func TestLineConnectsResampledHistory(t *testing.T) {
	t.Parallel()

	now := time.Unix(1000, 0)
	points := []timeseries.Point{
		{Time: now.Add(-8 * time.Second), Value: 0},
		{Time: now.Add(-4 * time.Second), Value: 100},
		{Time: now.Add(-2 * time.Second), Value: 0},
	}
	for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
		values := timeseries.Resample(points, now, 10*time.Second, chart.Slots(40, glyphs))
		grid := chart.Line{
			Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs,
		}.Render(40, 12)
		if got := components(grid, glyphs); got != 1 {
			t.Fatalf("%s: resampled history has %d components:\n%s", glyphs, got, show(grid))
		}
	}
}

func FuzzLineContinuity(f *testing.F) {
	f.Add([]byte{0, 100, 0, 100}, uint8(12), uint8(6))
	f.Add([]byte{0}, uint8(1), uint8(1))
	f.Add([]byte{100, 100, 0, 0}, uint8(2), uint8(31))
	f.Fuzz(func(t *testing.T, data []byte, w, h uint8) {
		if len(data) == 0 {
			return
		}
		width, height := 1+int(w%40), 1+int(h%20)
		for _, glyphs := range []chart.Glyphs{chart.Braille, chart.Block, chart.ASCII} {
			values := make([]float64, chart.Slots(width, glyphs))
			for i := range values {
				values[i] = float64(data[i%len(data)])
			}
			for _, fill := range []bool{false, true} {
				grid := chart.Line{
					Series: [][]float64{values}, Range: chart.Range{Max: 100}, Glyphs: glyphs, Fill: fill,
				}.Render(width, height)
				if got := components(grid, glyphs); got != 1 {
					t.Fatalf("%s fill=%t: got %d components:\n%s", glyphs, fill, got, show(grid))
				}
			}
		}
	})
}

func dots(grid chart.Grid, glyphs chart.Glyphs) map[image.Point]bool {
	points := make(map[image.Point]bool)
	bits := [4][2]rune{{1, 8}, {2, 16}, {4, 32}, {64, 128}}
	for y, row := range grid {
		for x, cell := range row {
			if cell.Series < 0 {
				continue
			}
			if glyphs != chart.Braille {
				points[image.Pt(x, y)] = true
				continue
			}
			for dy := range 4 {
				for dx := range 2 {
					if (cell.Rune-0x2800)&bits[dy][dx] != 0 {
						points[image.Pt(x*2+dx, y*4+dy)] = true
					}
				}
			}
		}
	}
	return points
}

func components(grid chart.Grid, glyphs chart.Glyphs) int {
	points := dots(grid, glyphs)
	total := 0
	for start := range points {
		total++
		delete(points, start)
		queue := []image.Point{start}
		for len(queue) > 0 {
			point := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					next := point.Add(image.Pt(dx, dy))
					if points[next] {
						delete(points, next)
						queue = append(queue, next)
					}
				}
			}
		}
	}
	return total
}
