// Package conf loads, validates, and writes hytop settings.
package conf

import (
	"image/color"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type Config struct {
	General    General             `toml:"general"`
	Appearance Appearance          `toml:"appearance"`
	Graphs     Graphs              `toml:"graphs"`
	Processes  Processes           `toml:"processes"`
	Logs       Logs                `toml:"logs"`
	Filters    map[string]string   `toml:"filters"`
	Theme      map[string]string   `toml:"theme"`
	Keys       map[string][]string `toml:"keys"`
	Pages      []Page              `toml:"page,omitempty"`
}

type Appearance struct {
	Contrast   string `toml:"contrast"`
	Background string `toml:"background"`
}

type General struct {
	Interval Duration `toml:"interval"`
	Span     Duration `toml:"span"`
}

type Graphs struct {
	Glyphs string `toml:"glyphs"`
	Colors string `toml:"colors"`
	Device string `toml:"device"`
}

type Processes struct {
	Sort       string   `toml:"sort"`
	Descending bool     `toml:"descending"`
	Tree       bool     `toml:"tree"`
	Kernel     bool     `toml:"kernel"`
	Filter     string   `toml:"filter"`
	Columns    []string `toml:"columns"`
}

type Logs struct {
	Lines int `toml:"lines"`
}

func (c Config) Clone() Config {
	cloned := c
	cloned.Processes.Columns = slices.Clone(c.Processes.Columns)
	cloned.Filters = maps.Clone(c.Filters)
	cloned.Theme = maps.Clone(c.Theme)
	cloned.Keys = make(map[string][]string, len(c.Keys))
	for name, keys := range c.Keys {
		cloned.Keys[name] = slices.Clone(keys)
	}
	cloned.Pages = clonePages(c.Pages)

	return cloned
}

func clonePages(pages []Page) []Page {
	cloned := make([]Page, len(pages))
	for i, page := range pages {
		cloned[i] = page
		cloned[i].Rows = cloneRows(page.Rows)
	}

	return cloned
}

func cloneRows(rows []Row) []Row {
	cloned := make([]Row, len(rows))
	for i, row := range rows {
		cloned[i] = row
		cloned[i].Children = clonePanes(row.Children)
	}

	return cloned
}

func clonePanes(panes []Pane) []Pane {
	cloned := make([]Pane, len(panes))
	for i, pane := range panes {
		cloned[i] = pane
		cloned[i].Rows = cloneRows(pane.Rows)
		cloned[i].Series = slices.Clone(pane.Series)
		cloned[i].Tokens = slices.Clone(pane.Tokens)
		cloned[i].Threshold = slices.Clone(pane.Threshold)
		cloned[i].Fill = clonePointer(pane.Fill)
		cloned[i].Axis = clonePointer(pane.Axis)
		cloned[i].Min = clonePointer(pane.Min)
		cloned[i].Max = clonePointer(pane.Max)
	}

	return cloned
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}

	cloned := *value

	return &cloned
}

const (
	MinInterval = 100 * time.Millisecond
	MaxSpan     = 24 * time.Hour
	MaxLogLines = 10_000
)

// Default returns the base configuration before file and flag overrides.
func Default() Config {
	return Config{
		General: General{
			Interval: Duration(time.Second),
			Span:     Duration(time.Minute),
		},
		Appearance: Appearance{
			Contrast:   Auto,
			Background: Auto,
		},
		Graphs: Graphs{
			Glyphs: Auto,
			Colors: Auto,
			Device: "cpu",
		},
		Processes: Processes{
			Sort:       "cpu",
			Descending: true,
			Columns:    procmodel.DefaultColumns(),
		},
		Logs: Logs{Lines: 300},
	}
}

// Auto leaves a setting to whatever the terminal reports.
const Auto = "auto"

const (
	ContrastStandard = "standard"
	ContrastHigh     = "high"
	BackgroundDark   = "dark"
	BackgroundLight  = "light"
)

// Caps applies configured graph capabilities to detected capabilities.
func (c Config) Caps(detected render.Caps) render.Caps {
	if glyphs, ok := render.ParseGlyphs(c.Graphs.Glyphs); ok {
		detected.Glyphs = glyphs
	}

	if colors, ok := render.ParseColors(c.Graphs.Colors); ok {
		detected.Colors = colors
	}

	return detected
}

// Palette resolves configured theme tokens.
func (c Config) Palette() map[theme.Token]color.Color {
	if len(c.Theme) == 0 {
		return nil
	}

	palette := make(map[theme.Token]color.Color, len(c.Theme))

	for name, value := range c.Theme {
		token, known := theme.ParseToken(name)
		if !known {
			continue
		}

		if strings.EqualFold(value, "default") || strings.EqualFold(value, "inherit") {
			palette[token] = lipgloss.NoColor{}
		} else {
			palette[token] = lipgloss.Color(value)
		}
	}

	return palette
}

// ThemeOptions combines a configured background override with a terminal
// background detected at runtime.
func (c Config) ThemeOptions(detectedDark *bool) theme.Options {
	dark := detectedDark

	switch c.Appearance.Background {
	case BackgroundDark:
		value := true
		dark = &value
	case BackgroundLight:
		value := false
		dark = &value
	}

	return theme.Options{
		HighContrast: c.Appearance.Contrast == ContrastHigh,
		Muted:        c.Appearance.Contrast == ContrastStandard,
		Dark:         dark,
	}
}

// Keymap applies configured bindings over the defaults.
func (c Config) Keymap() *keymap.Map {
	bound, err := keymap.Default().Rebind(c.Keys)
	if err != nil {
		return keymap.Default()
	}

	return bound
}

// Columns returns the configured process-table layout.
func (c Config) Columns() []procmodel.Column {
	resolved, err := procmodel.ResolveColumns(c.Processes.Columns)
	if err != nil {
		resolved, _ = procmodel.ResolveColumns(nil)
	}

	return resolved
}

// Filter resolves a named filter or compiles nameOrExpr directly.
func (c Config) Filter(nameOrExpr string) (string, filter.Expr, error) {
	if nameOrExpr == "" {
		return "", nil, nil
	}

	source := nameOrExpr
	if named, ok := c.Filters[nameOrExpr]; ok {
		source = named
	}

	expr, err := filter.Compile(source)
	if err != nil {
		return "", nil, errors.Wrapf(err, "filter %q", nameOrExpr)
	}

	return source, expr, nil
}

// Validate reports the first invalid setting.
func (c Config) Validate() error {
	if time.Duration(c.General.Interval) < MinInterval {
		return errors.Newf("general.interval must be at least %s, got %s", MinInterval, c.General.Interval)
	}

	if time.Duration(c.General.Span) < time.Duration(c.General.Interval) {
		return errors.Newf("general.span (%s) is shorter than general.interval (%s)",
			c.General.Span, c.General.Interval)
	}

	if time.Duration(c.General.Span) > MaxSpan {
		return errors.Newf("general.span must not exceed %s, got %s", MaxSpan, c.General.Span)
	}

	if err := c.validateDisplay(); err != nil {
		return err
	}

	if err := oneOf("processes.sort", c.Processes.Sort, []string{"cpu", "rss", "pid", "name"}); err != nil {
		return err
	}

	if _, err := procmodel.ResolveColumns(c.Processes.Columns); err != nil {
		return errors.Wrap(err, "processes.columns")
	}

	if c.Logs.Lines < 1 {
		return errors.Newf("logs.lines must be at least 1, got %d", c.Logs.Lines)
	}

	if c.Logs.Lines > MaxLogLines {
		return errors.Newf("logs.lines must not exceed %d, got %d", MaxLogLines, c.Logs.Lines)
	}

	for name, source := range c.Filters {
		if _, err := filter.Compile(source); err != nil {
			return errors.Wrapf(err, "filters.%s", name)
		}
	}

	if _, _, err := c.Filter(c.Processes.Filter); err != nil {
		return errors.Wrap(err, "processes.filter")
	}

	if _, err := keymap.Default().Rebind(c.Keys); err != nil {
		return errors.Wrap(err, "keys")
	}

	for name, value := range c.Theme {
		if _, known := theme.ParseToken(name); !known {
			return errors.Newf("theme.%s is not a token; known ones are %v", name, theme.TokenNames())
		}

		if value == "" {
			return errors.Newf("theme.%s is empty", name)
		}
		if strings.EqualFold(value, "default") || strings.EqualFold(value, "inherit") {
			continue
		}
		if _, invalid := lipgloss.Color(value).(lipgloss.NoColor); invalid {
			return errors.Newf("theme.%s %q is not a valid color", name, value)
		}
	}

	if err := c.validatePages(); err != nil {
		return err
	}

	return nil
}

func (c Config) validateDisplay() error {
	checks := []struct {
		key     string
		value   string
		allowed []string
	}{
		{"appearance.contrast", c.Appearance.Contrast, []string{Auto, ContrastStandard, ContrastHigh}},
		{"appearance.background", c.Appearance.Background, []string{Auto, BackgroundDark, BackgroundLight}},
		{"graphs.glyphs", c.Graphs.Glyphs, append(render.GlyphNames(), Auto)},
		{"graphs.colors", c.Graphs.Colors, append(render.ColorNames(), Auto)},
		{"graphs.device", c.Graphs.Device, []string{"cpu", "memory", "disk", "network", "sensors"}},
	}

	for _, check := range checks {
		if err := oneOf(check.key, check.value, check.allowed); err != nil {
			return err
		}
	}

	return nil
}

func oneOf(key, value string, allowed []string) error {
	for _, candidate := range allowed {
		if candidate == value {
			return nil
		}
	}

	return errors.Newf("%s is %q; it must be one of %v", key, value, allowed)
}

// Duration is a TOML string, since TOML has no duration of its own.
type Duration time.Duration

func (d Duration) String() string { return series.Span(time.Duration(d)) }

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return errors.Wrapf(err, "%q is not a duration", text)
	}

	*d = Duration(parsed)

	return nil
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(d.String()), nil }
