package summary

import (
	"strings"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// Services renders the systemd summary above the unit list.
type Services struct {
	reactea.BasicComponent

	base
}

func NewServices(s *store.Store, t *theme.Theme, caps render.Caps) *Services {
	return &Services{base: base{store: s, theme: t, caps: caps}}
}

func (w *Services) Render(ctx *reactea.Ctx) string {
	width := ctx.Width()
	if width <= 0 {
		return ""
	}

	return strings.Join([]string{
		w.line(width, w.units(), w.state()),
		w.line(width, w.work(), w.boot()),
	}, "\n")
}

func (w *Services) units() string {
	parts := []string{
		w.count("units", "units.total", theme.Text),
		w.count("running", "units.running", theme.Own),
		w.count("active", "units.active", theme.Dim),
	}

	parts = append(parts, w.count("failed", "units.failed", w.failedToken()))

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
}

func (w *Services) failedToken() theme.Token {
	if value, ok := w.store.Last("units.failed"); ok && value.Value > 0 {
		return theme.Critical
	}

	return theme.Dim
}

func (w *Services) state() string {
	text, ok := w.store.Fact(series.FactUnitsState)
	if !ok {
		return ""
	}

	token := theme.Own

	switch text {
	case "degraded", "maintenance":
		token = theme.Critical
	case "starting", "stopping", "initializing":
		token = theme.Warn
	}

	return w.theme.Style(theme.Label).Render("systemd ") + w.theme.Style(token).Render(text)
}

func (w *Services) work() string {
	parts := make([]string, 0, 4)

	for _, kind := range []struct {
		label string
		key   series.Key
	}{
		{"services", "units.service"},
		{"timers", "units.timer"},
		{"sockets", "units.socket"},
	} {
		if value, ok := w.store.Last(kind.key); ok && value.Value > 0 {
			parts = append(parts, w.count(kind.label, kind.key, theme.Dim))
		}
	}

	jobs, ok := w.store.Last("units.jobs")
	if !ok {
		return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
	}

	token := theme.Dim
	if jobs.Value > 0 {
		token = theme.Warn
	}

	parts = append(parts, w.count("jobs queued", "units.jobs", token))

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
}

func (w *Services) boot() string {
	took, ok := w.store.Fact(series.FactUnitsBoot)
	if !ok {
		return ""
	}

	return w.theme.Style(theme.Label).Render("userspace came up in ") +
		w.theme.Style(theme.Text).Render(took)
}
