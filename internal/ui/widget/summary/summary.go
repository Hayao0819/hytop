// Package summary draws the shared two-row heading above list pages.
package summary

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Rows is the fixed summary height.
const Rows = 2

type base struct {
	store *store.Store
	theme *theme.Theme
	caps  render.Caps
}

type Widget struct {
	reactea.BasicComponent

	base
}

func New(s *store.Store, t *theme.Theme, caps render.Caps) *Widget {
	return &Widget{base: base{store: s, theme: t, caps: caps}}
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	return w.render(ctx, func(width int) [][2]string {
		return [][2]string{
			{w.tasks(), w.timing()},
			{w.meters(width), w.io(transfer)},
		}
	})
}

func (w base) render(ctx *reactea.Ctx, content func(int) [][2]string) string {
	width := ctx.Width()
	if width <= 0 {
		return ""
	}

	rows := content(width)
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, w.line(width, row[0], row[1]))
	}

	return strings.Join(lines, "\n")
}

func (w base) line(width int, left, right string) string {
	clip := lipgloss.NewStyle().MaxWidth(width)

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 2 {
		return clip.Render(" " + left)
	}

	return clip.Render(" " + left + strings.Repeat(" ", gap) + right + " ")
}

func (w base) tasks() string {
	parts := []string{
		w.count("Tasks", "proc.count", theme.Text),
		w.count("running", "proc.running", theme.Own),
		w.count("sleeping", "proc.sleeping", theme.Dim),
	}

	for _, rare := range []struct {
		label string
		key   series.Key
		token theme.Token
	}{
		{"stopped", "proc.stopped", theme.Warn},
		{"zombie", "proc.zombie", theme.Critical},
		{"blocked", "proc.blocked", theme.Warn},
	} {
		if value, ok := w.store.Last(rare.key); ok && value.Value > 0 {
			parts = append(parts, w.count(rare.label, rare.key, rare.token))
		}
	}

	parts = append(parts, w.count("threads", "proc.threads", theme.Dim))

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
}

func (w base) count(label string, key series.Key, token theme.Token) string {
	value, ok := w.store.Last(key)
	if !ok {
		return ""
	}

	return w.theme.Style(token).Bold(true).Render(fmt.Sprintf("%.0f", value.Value)) +
		" " + w.theme.Style(theme.Label).Render(label)
}

func (w base) timing() string {
	var parts []string

	if loads := w.loads(); loads != "" {
		parts = append(parts, w.theme.Style(theme.Label).Render("load ")+loads)
	}

	if up, ok := w.store.Last("system.uptime"); ok {
		parts = append(parts, w.theme.Style(theme.Label).Render("up ")+
			w.theme.Style(theme.Text).Render(series.Duration.Format(up.Value, 0, series.Auto)))
	}

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("   "))
}

// loads normalizes alert thresholds by logical processor count.
func (w base) loads() string {
	cores := w.cores()
	parts := make([]string, 0, 3)

	for _, key := range []series.Key{"load.1", "load.5", "load.15"} {
		value, ok := w.store.Last(key)
		if !ok {
			return ""
		}

		token := theme.Text
		if cores > 0 {
			token = w.theme.Threshold(value.Value/cores, 0.7, 1)
		}

		parts = append(parts, w.theme.Style(token).Render(fmt.Sprintf("%.2f", value.Value)))
	}

	return strings.Join(parts, " ")
}

func (w base) cores() float64 {
	logical, ok := w.store.Fact(series.FactCPULogical)
	if !ok {
		return 0
	}

	var count float64
	if _, err := fmt.Sscanf(logical, "%f", &count); err != nil {
		return 0
	}

	return count
}

func (w base) meters(width int) string {
	span := min(max((width-56)/3, 6), 20)

	parts := []string{
		w.meter("CPU", "cpu.total.usage", theme.CPU, span, ""),
		w.meter("Memory", "mem.usage", theme.Memory, span, w.ratio("mem.used", "mem.total")),
	}

	if used, ok := w.store.Last("swap.used"); ok && used.Value > 0 {
		parts = append(parts, w.meter("Swap", "", theme.Swap, span,
			w.ratio("swap.used", "swap.total")))
	}

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("   "))
}

func (w base) meter(label string, key series.Key, token theme.Token, span int, note string) string {
	percent, ok := w.percent(key)
	if !ok {
		return ""
	}

	return w.gauge(label, percent, token, span, note)
}

func gaugeOf(percent float64, span int, caps render.Caps) (string, string) {
	return chart.Gauge(percent/100, span, caps.Glyphs)
}

func (w base) percent(key series.Key) (float64, bool) {
	if key != "" {
		value, ok := w.store.Last(key)

		return value.Value, ok
	}

	used, okUsed := w.store.Last("swap.used")
	total, okTotal := w.store.Last("swap.total")

	if !okUsed || !okTotal || total.Value <= 0 {
		return 0, false
	}

	return 100 * used.Value / total.Value, true
}

func (w base) ratio(used, total series.Key) string {
	a, okA := w.store.Last(used)
	b, okB := w.store.Last(total)

	if !okA || !okB {
		return ""
	}

	return series.Bytes.Format(a.Value, 1, series.Auto) + " / " + series.Bytes.Format(b.Value, 1, series.Auto)
}

type flow struct {
	label string
	key   series.Key
	token theme.Token
}

var (
	transfer = []flow{
		{"read", "diskio.total.read", theme.DiskRead},
		{"write", "diskio.total.write", theme.DiskWrite},
		{"rx", "net.total.rx", theme.NetRx},
		{"tx", "net.total.tx", theme.NetTx},
	}

	storage = transfer[:2]
)

func (w base) io(flows []flow) string {
	var parts []string

	for _, flow := range flows {
		value, ok := w.store.Last(flow.key)
		if !ok {
			continue
		}

		parts = append(parts, w.theme.Style(theme.Label).Render(flow.label+" ")+
			w.theme.Style(flow.token).Render(series.BytesPerSecond.Format(value.Value, 1, series.Auto)))
	}

	return strings.Join(parts, w.theme.Style(theme.Dim).Render("  "))
}
