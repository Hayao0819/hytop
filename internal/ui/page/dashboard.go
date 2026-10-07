package page

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"
	"github.com/charmbracelet/x/ansi"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	gaugewidget "github.com/Hayao0819/hytop/internal/ui/widget/gauge"
	"github.com/Hayao0819/hytop/internal/ui/widget/graph"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Dashboard compiles a configured page into a fixed reactea component tree.
type Dashboard struct {
	reactea.Wrapper

	keys    *keymap.Map
	boxes   []*layout.Box
	problem string
}

func NewDashboard(env Env, spec conf.Page) *Dashboard {
	d := &Dashboard{keys: env.Keys}

	component, err := d.rows(env, spec.Rows, "page."+spec.Name)
	if err != nil {
		d.problem = err.Error()
		component = reactea.Text(safe.Text(d.problem))
	}

	d.Wrapper = reactea.Wrap(component)

	return d
}

func (d *Dashboard) Hints() []Hint {
	for _, box := range d.boxes {
		if len(box.Starved()) > 0 {
			return []Hint{{Key: "!", What: "the window is too small for this layout"}}
		}
	}

	return d.keys.Hints(keymap.Global)
}

func (d *Dashboard) Error() string { return d.problem }

func (d *Dashboard) rows(env Env, rows []conf.Row, path string) (reactea.Component, error) {
	items := make([]layout.Item, 0, len(rows))

	for i, row := range rows {
		children := make([]layout.Item, 0, len(row.Children))
		for j, pane := range row.Children {
			component, err := d.pane(env, pane, fmt.Sprintf("%s.row[%d].child[%d]", path, i, j))
			if err != nil {
				return nil, err
			}

			minimum, maximum := pane.MinWidth, pane.MaxWidth
			if minimum == 0 {
				minimum = minimumWidth(pane)
			}
			item := layout.Grow(ratio(pane.Ratio), component).
				Bounds(minimum, maximum).
				MinCross(max(pane.MinHeight, minimumHeight(pane))).
				Key(fmt.Sprintf("%s/%d/%d", path, i, j))
			if len(pane.Rows) == 0 {
				item = item.Focusable()
			}
			children = append(children, item)
		}

		box := layout.Row(children...)
		d.boxes = append(d.boxes, box)

		item := layout.Grow(ratio(row.Ratio), box).
			Bounds(row.MinHeight, row.MaxHeight).
			MinCross(row.MinWidth).
			Key(fmt.Sprintf("%s/%d", path, i))
		items = append(items, item)
	}

	box := layout.Column(items...)
	d.boxes = append(d.boxes, box)

	return box, nil
}

func (d *Dashboard) pane(env Env, spec conf.Pane, path string) (reactea.Component, error) {
	if len(spec.Rows) > 0 {
		return d.rows(env, spec.Rows, path)
	}

	component, err := dashboardWidget(env, spec)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if spec.Title != "" {
		title := safe.Text(spec.Title)
		component = layout.Column(
			layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
				return env.Theme.Style(theme.Heading).MaxWidth(ctx.Width()).Render(title)
			})),
			layout.Grow(1, component),
		)
	}

	if spec.Border == "none" {
		return component, nil
	}

	style := lipgloss.NewStyle().
		BorderStyle(dashboardBorder(spec.Border)).
		BorderForeground(env.Theme.Colour(theme.Border)).
		Padding(0, 1)
	active := style.BorderForeground(env.Theme.Colour(theme.BorderActive))

	return layout.Framed(style, component).WhenFocused(active), nil
}

type dashboardWidgetBuilder func(Env, conf.Pane) (reactea.Component, error)

var dashboardWidgetBuilders = map[conf.WidgetKind]dashboardWidgetBuilder{
	conf.WidgetLine:      dashboardGraph,
	conf.WidgetBraille:   dashboardGraph,
	conf.WidgetSparkline: dashboardGraph,
	conf.WidgetGauge:     dashboardGauge,
	conf.WidgetText:      dashboardText,
}

func dashboardWidget(env Env, spec conf.Pane) (reactea.Component, error) {
	build, ok := dashboardWidgetBuilders[spec.Widget]
	if !ok {
		return nil, fmt.Errorf("unknown widget %q", spec.Widget)
	}

	return build(env, spec)
}

func dashboardText(_ Env, spec conf.Pane) (reactea.Component, error) {
	text := safe.Text(spec.Text)

	return reactea.Func(func(ctx *reactea.Ctx) string {
		wrapped := ansi.Wordwrap(text, ctx.Width(), " ")

		return lipgloss.NewStyle().MaxWidth(ctx.Width()).MaxHeight(ctx.Height()).Render(wrapped)
	}), nil
}

func dashboardGraph(env Env, spec conf.Pane) (reactea.Component, error) {
	patterns := spec.Series
	if len(patterns) == 0 {
		return nil, fmt.Errorf("%s widget has no series", spec.Widget)
	}
	tracks := make([]graph.Track, 0, len(patterns))
	for i, source := range patterns {
		pattern, err := series.ParsePattern(source)
		if err != nil {
			return nil, err
		}

		token := dashboardToken(i)
		if len(spec.Tokens) > 0 {
			token, _ = theme.ParseToken(spec.Tokens[i])
		}
		track := graph.Track{Pattern: pattern, Colour: env.Theme.Colour(token)}
		if len(spec.Tokens) == 0 && strings.Contains(source, "*") {
			track.Palette = dashboardPalette(env.Theme, i)
		}
		tracks = append(tracks, track)
	}

	caps := env.Caps
	if spec.Widget != conf.WidgetBraille && caps.Glyphs == render.Braille {
		caps.Glyphs = render.Block
	}

	g := graph.New(env.Store, caps, tracks...)
	g.Span = env.Span
	if spec.History > 0 {
		g.Span = time.Duration(spec.History)
	}
	g.Unit = series.None
	if env.Registry != nil {
		g.Unit, _ = env.Registry.PatternUnit(tracks[0].Pattern)
	}
	g.Scale, _ = series.ParseScale(spec.Unit)
	g.Precision = spec.Precision
	if spec.Max != nil {
		minimum := 0.0
		if spec.Min != nil {
			minimum = *spec.Min
		}
		g.Range = chart.Range{Min: minimum, Max: *spec.Max}
	} else if g.Unit != series.Percent {
		g.Range = chart.Range{}
	}
	if spec.Widget == conf.WidgetSparkline {
		g.Axis, g.Fill = false, false
		g.MaxHeight = 1
	} else {
		if spec.Axis != nil {
			g.Axis = *spec.Axis
		}
		if spec.Fill != nil {
			g.Fill = *spec.Fill
		}
	}

	return layout.Memo(g, func() any { return env.Store.Generation() }), nil
}

func dashboardGauge(env Env, spec conf.Pane) (reactea.Component, error) {
	key, err := series.ParseKey(spec.Series[0])
	if err != nil {
		return nil, err
	}

	token := dashboardToken(0)
	if len(spec.Tokens) > 0 {
		token, _ = theme.ParseToken(spec.Tokens[0])
	}

	widget := gaugewidget.New(env.Store, env.Theme, env.Caps, key)
	widget.Token = token
	widget.Scale, _ = series.ParseScale(spec.Unit)
	widget.Precision = spec.Precision
	widget.Threshold = append([]float64(nil), spec.Threshold...)
	if env.Registry != nil {
		widget.Unit = env.Registry.Unit(key)
	}
	if spec.Min != nil {
		widget.Min = *spec.Min
	}
	if spec.Max != nil {
		widget.Max = *spec.Max
	}

	return layout.Memo(widget, func() any { return env.Store.Generation() }), nil
}

func dashboardBorder(name string) lipgloss.Border {
	switch name {
	case "plain":
		return lipgloss.NormalBorder()
	case "double":
		return lipgloss.DoubleBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}

var dashboardTokens = [...]theme.Token{
	theme.CPU, theme.Memory, theme.NetRx, theme.DiskRead,
	theme.CPUPressure, theme.Swap, theme.NetTx, theme.DiskWrite,
}

func dashboardToken(index int) theme.Token {
	return dashboardTokens[index%len(dashboardTokens)]
}

func dashboardPalette(t *theme.Theme, offset int) []color.Color {
	colours := make([]color.Color, len(dashboardTokens))
	for i := range colours {
		colours[i] = t.Colour(dashboardToken(i + offset))
	}

	return colours
}

func ratio(value int) int {
	if value > 0 {
		return value
	}

	return 1
}

func minimumWidth(spec conf.Pane) int {
	if len(spec.Rows) > 0 {
		return 1
	}
	if spec.Widget == conf.WidgetText {
		return 4
	}

	return 12
}

func minimumHeight(spec conf.Pane) int {
	if len(spec.Rows) > 0 {
		return 1
	}
	height := 1
	if spec.Widget != conf.WidgetText && spec.Widget != conf.WidgetSparkline {
		height = 3
	}
	if spec.Title != "" {
		height++
	}
	if spec.Border != "none" {
		height += 2
	}

	return height
}

func DashboardTitle(spec conf.Page) string {
	if strings.TrimSpace(spec.Title) != "" {
		return safe.Text(spec.Title)
	}

	return safe.Text(spec.Name)
}
