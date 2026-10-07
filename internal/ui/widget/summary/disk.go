package summary

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// Disk renders the summary shared by storage pages.
type Disk struct {
	reactea.BasicComponent

	base

	mounts func() []diskmodel.Mount
	drives func() []diskmodel.Device
}

func NewDisk(
	s *store.Store, t *theme.Theme, caps render.Caps,
	mounts func() []diskmodel.Mount, drives func() []diskmodel.Device,
) *Disk {
	if mounts == nil {
		mounts = func() []diskmodel.Mount { return nil }
	}

	if drives == nil {
		drives = func() []diskmodel.Device { return nil }
	}

	return &Disk{base: base{store: s, theme: t, caps: caps}, mounts: mounts, drives: drives}
}

func (w *Disk) Render(ctx *reactea.Ctx) string {
	return w.render(ctx, func(width int) [][2]string {
		return [][2]string{
			{w.hardware(), w.health()},
			{w.space(width), w.io(storage)},
		}
	})
}

func (w *Disk) hardware() string {
	var (
		drives = w.drives()
		mounts = w.mounts()
		rotate int
		solid  int
	)

	var optical int

	for _, drive := range drives {
		switch {
		case drive.Optical:
			optical++
		case drive.Rotating:
			rotate++
		default:
			solid++
		}
	}

	parts := []string{w.plain("drives", len(drives), theme.Text)}

	for _, kind := range []struct {
		label string
		count int
	}{
		{"solid state", solid},
		{"spinning", rotate},
		{"optical", optical},
	} {
		if kind.count > 0 {
			parts = append(parts, w.plain(kind.label, kind.count, theme.Dim))
		}
	}

	parts = append(parts, w.plain("filesystems", len(mounts), theme.Dim))

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
}

// health keeps unread SMART status distinct from a passing result.
func (w *Disk) health() string {
	drives := w.drives()
	if len(drives) == 0 {
		return ""
	}

	var failing, unknown int

	for _, drive := range drives {
		switch drive.Health {
		case diskmodel.Failing:
			failing++
		case diskmodel.Unknown:
			unknown++
		}
	}

	switch {
	case failing > 0:
		return w.theme.Style(theme.Critical).Render(
			fmt.Sprintf("%d drive%s failing SMART", failing, plural(failing)))

	case unknown > 0:
		return w.theme.Style(theme.Label).Render(
			fmt.Sprintf("SMART unread on %d drive%s", unknown, plural(unknown)))

	default:
		return w.theme.Style(theme.Label).Render("SMART ") + w.theme.Style(theme.Own).Render("all passing")
	}
}

func (w *Disk) space(width int) string {
	used, okUsed := w.store.Last("fs.total.used")
	size, okSize := w.store.Last("fs.total.size")

	if !okUsed || !okSize || size.Value <= 0 {
		return ""
	}

	var (
		share = 100 * used.Value / size.Value
		span  = min(max((width-52)/2, 8), 24)
	)

	return w.gauge("Storage", share, theme.DiskWrite, span,
		series.Bytes.Format(used.Value, 1, series.Auto)+" of "+
			series.Bytes.Format(size.Value, 1, series.Auto)) +
		w.theme.Style(theme.Dim).Render("   ") + w.fullest()
}

func (w *Disk) fullest() string {
	var (
		worst diskmodel.Mount
		found bool
	)

	for _, mount := range w.mounts() {
		if mount.Total == 0 {
			continue
		}

		if !found || mount.Usage() > worst.Usage() {
			worst, found = mount, true
		}
	}

	if !found {
		return ""
	}

	return w.theme.Style(theme.Label).Render("fullest ") +
		w.theme.Style(theme.Text).Render(safe.Text(worst.Path)) + " " +
		w.theme.Style(w.theme.Threshold(worst.Usage(), 75, 90)).
			Render(fmt.Sprintf("%.0f %%", worst.Usage()))
}

func (w base) plain(label string, value int, token theme.Token) string {
	return w.theme.Style(token).Bold(true).Render(fmt.Sprint(value)) +
		" " + w.theme.Style(theme.Label).Render(label)
}

func (w base) gauge(label string, percent float64, token theme.Token, span int, note string) string {
	filled, rest := gaugeOf(percent, span, w.caps)

	out := w.theme.Style(theme.Label).Render(label+" ") +
		w.theme.Style(token).Render(filled) +
		w.theme.Style(theme.Dim).Render(rest) +
		" " + w.theme.Style(theme.Text).Bold(true).Render(fmt.Sprintf("%4.1f %%", percent))

	if note != "" {
		out += " " + w.theme.Style(theme.Label).Render(note)
	}

	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}
