package graph

import (
	"image/color"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

func TestPatternCyclesThroughItsPalette(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	memory.WriteSamples([]metric.Sample{
		{Key: "cpu.core.0.usage", Value: 10, Time: time.Now()},
		{Key: "cpu.core.1.usage", Value: 20, Time: time.Now()},
	})
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	widget := New(memory, render.Caps{}, Track{
		Pattern: "cpu.core.*.usage",
		Palette: []color.Color{red, blue},
	})

	resolved := widget.resolveTracks()
	if len(resolved) != 2 {
		t.Fatalf("resolved tracks = %v, want two", resolved)
	}
	if resolved[0].Colour != red || resolved[1].Colour != blue {
		t.Errorf("resolved colours = %v, %v", resolved[0].Colour, resolved[1].Colour)
	}
}

func TestLegendUsesResolvedSeriesColours(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	memory.WriteSamples([]metric.Sample{
		{Key: "diskio.nvme0n1.util", Value: 20, Time: time.Now()},
		{Key: "diskio.sda.util", Value: 40, Time: time.Now()},
	})
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	caps := render.Caps{Colors: render.TrueColor}
	w := New(memory, caps, Track{
		Pattern: "diskio.*.util", Palette: []color.Color{red, blue},
	})
	w.Legend = func(key series.Key) string { return key.Segments()[1] }
	want := render.Foreground(caps, red).Render("nvme0n1") + "  " +
		render.Foreground(caps, blue).Render("sda")
	if got := w.legend(w.resolveTracks(), 80); got != want {
		t.Fatalf("legend = %q, want %q", got, want)
	}
}

func TestLegendClipsByCellsAndSanitizesLabels(t *testing.T) {
	t.Parallel()

	for _, label := range []string{"disk\n\x1b[31mname", "ディスク0"} {
		w := New(store.New(nil), render.Caps{Colors: render.Mono})
		w.Legend = func(series.Key) string { return label }
		for _, width := range []int{0, 1, 8, 30} {
			got := w.legend([]Track{{Key: "diskio.sda.util"}}, width)
			if !utf8.ValidString(got) || strings.ContainsAny(got, "\n\r\x1b") || lipgloss.Width(got) > width {
				t.Fatalf("legend at width %d = %q", width, got)
			}
		}
	}
}
