// Package graph binds a store series to a chart and paints it.
package graph

import (
	"image/color"
	"math"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/widget"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Track is one line on the graph.
type Track struct {
	Key     series.Key
	Pattern series.Pattern
	Colour  color.Color
	Palette []color.Color
}

type Widget struct {
	reactea.BasicComponent

	store  *store.Store
	tracks []Track
	caps   render.Caps

	Span  time.Duration
	Range chart.Range
	Fill  bool
	Axis  bool
	Unit  series.Unit
	Scale series.Scale
	// Precision applies to both axis labels.
	Precision int
	Label     string
	MaxHeight int

	extent chart.Range
}

func New(s *store.Store, caps render.Caps, tracks ...Track) *Widget {
	return &Widget{
		store:  s,
		tracks: tracks,
		caps:   caps,
		Span:   time.Minute,
		Range:  chart.Range{Max: 100},
		Fill:   true,
		Axis:   true,
		Unit:   series.Percent,
	}
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if w.MaxHeight > 0 {
		height = min(height, w.MaxHeight)
	}
	if width <= 0 || height <= 0 {
		return ""
	}

	var (
		slots    = chart.Slots(width, w.caps.Glyphs)
		now      = time.Now()
		resolved = w.resolveTracks()
		values   = make([][]float64, 0, len(resolved))
		colours  = make([]color.Color, 0, len(resolved))
		hasData  bool
	)

	for _, track := range resolved {
		window := w.store.Window(track.Key, w.Span)
		resampled := widget.Resample(window, now, w.Span, slots)
		for _, value := range resampled {
			if !math.IsNaN(value) && !math.IsInf(value, 0) {
				hasData = true

				break
			}
		}
		values = append(values, resampled)
		colours = append(colours, track.Colour)
	}
	if !hasData {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, "no data")
	}

	line := chart.Line{
		Series: values,
		Range:  w.Range,
		Glyphs: w.caps.Glyphs,
		Fill:   w.Fill,
	}

	w.extent.Min, w.extent.Max = line.Extent()

	grid := line.Render(width, height)

	w.corners(grid, height)

	return w.paint(grid, colours)
}

func (w *Widget) resolveTracks() []Track {
	hasPattern := false
	for _, track := range w.tracks {
		if track.Pattern != "" {
			hasPattern = true

			break
		}
	}
	if !hasPattern {
		return w.tracks
	}

	live := w.store.Keys()
	seen := make(map[series.Key]bool, len(w.tracks))
	resolved := make([]Track, 0, len(w.tracks))

	for _, track := range w.tracks {
		keys := []series.Key{track.Key}
		if track.Pattern != "" {
			keys = track.Pattern.Expand(live)
		}

		for i, key := range keys {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			colour := track.Colour
			if len(track.Palette) > 0 {
				colour = track.Palette[i%len(track.Palette)]
			}
			resolved = append(resolved, Track{Key: key, Colour: colour})
		}
	}

	return resolved
}

// Fixed corner labels keep scrolling data from shifting the axis.
func (w *Widget) corners(grid chart.Grid, height int) {
	if !w.Axis || height < 2 {
		return
	}

	grid.Stamp(0, 0, w.Label)
	grid.StampRight(0, w.Unit.Format(w.extent.Max, w.Precision, w.Scale))
	grid.StampRight(height-1, w.Unit.Format(w.extent.Min, w.Precision, w.Scale))
	grid.Stamp(height-1, 0, series.Span(w.Span)+" ago")
}

func (w *Widget) paint(grid chart.Grid, colours []color.Color) string {
	dim := lipgloss.NewStyle().Faint(true)
	return grid.Render(func(owner chart.SeriesID) lipgloss.Style {
		switch {
		case owner == chart.Label:
			return dim
		case owner >= 0 && int(owner) < len(colours):
			return render.Foreground(w.caps, colours[int(owner)])
		default:
			return lipgloss.NewStyle()
		}
	})
}
