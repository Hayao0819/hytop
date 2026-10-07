package graph

import (
	"image/color"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
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
