// Package gauge draws the current value of one series as a compact bar.
package gauge

import (
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

type Widget struct {
	reactea.BasicComponent

	Store *store.Store
	Theme *theme.Theme
	Caps  render.Caps
	Key   series.Key
	Unit  series.Unit
	Scale series.Scale

	Precision int
	Min       float64
	Max       float64
	Threshold []float64
	Token     theme.Token
}

func New(memory *store.Store, t *theme.Theme, caps render.Caps, key series.Key) *Widget {
	return &Widget{
		Store: memory, Theme: t, Caps: caps, Key: key,
		Max: 100, Token: theme.CPU,
	}
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	value, label, token, marker := w.reading()
	filled, rest := chart.Gauge((value-w.Min)/(w.Max-w.Min), width, w.Caps.Glyphs)
	bar := w.Theme.Style(token).Render(filled) + w.Theme.Style(theme.Dim).Render(rest)
	reading := marker + label

	if height == 1 {
		labelWidth := lipgloss.Width(reading)
		barWidth := max(0, width-labelWidth-1)
		filled, rest = chart.Gauge((value-w.Min)/(w.Max-w.Min), barWidth, w.Caps.Glyphs)
		bar = w.Theme.Style(token).Render(filled) + w.Theme.Style(theme.Dim).Render(rest)
		if barWidth > 0 {
			bar += " "
		}

		return bar + reading
	}

	content := lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(reading) + "\n" + bar

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (w *Widget) reading() (float64, string, theme.Token, string) {
	point, ok := w.Store.Last(w.Key)
	if !ok {
		return w.Min, "-", theme.Dim, ""
	}

	token := w.Token
	marker := ""
	if len(w.Threshold) == 2 {
		switch w.Theme.Threshold(point.Value, w.Threshold[0], w.Threshold[1]) {
		case theme.Warn:
			token, marker = theme.Warn, "▲ "
		case theme.Critical:
			token, marker = theme.Critical, "◆ "
		}
	}

	return point.Value, w.Unit.Format(point.Value, w.Precision, w.Scale), token, marker
}
