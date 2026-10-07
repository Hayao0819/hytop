package conf

import (
	"fmt"
	"math"
	"unicode"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// Page is a user-defined top-level dashboard. Rows and panes alternate so the
// TOML mirrors the shape on screen without exposing the UI framework.
type Page struct {
	Name  string `toml:"name"`
	Title string `toml:"title,omitempty"`
	Rows  []Row  `toml:"row"`
}

type WidgetKind string

const (
	WidgetLine      WidgetKind = "line"
	WidgetBraille   WidgetKind = "braille"
	WidgetSparkline WidgetKind = "sparkline"
	WidgetGauge     WidgetKind = "gauge"
	WidgetText      WidgetKind = "text"
)

// Row divides its height among sibling rows and its width among Children.
type Row struct {
	Ratio     int    `toml:"ratio,omitempty"`
	MinHeight int    `toml:"min_height,omitempty"`
	MaxHeight int    `toml:"max_height,omitempty"`
	MinWidth  int    `toml:"min_width,omitempty"`
	Children  []Pane `toml:"child"`
}

// Pane is either a widget or another column of rows.
type Pane struct {
	Ratio     int `toml:"ratio,omitempty"`
	MinWidth  int `toml:"min_width,omitempty"`
	MaxWidth  int `toml:"max_width,omitempty"`
	MinHeight int `toml:"min_height,omitempty"`

	Rows []Row `toml:"row,omitempty"`

	Widget    WidgetKind `toml:"widget,omitempty"`
	Series    StringList `toml:"series,omitempty"`
	Tokens    StringList `toml:"tokens,omitempty"`
	Title     string     `toml:"title,omitempty"`
	Text      string     `toml:"text,omitempty"`
	Border    string     `toml:"border,omitempty"`
	History   Duration   `toml:"history,omitempty"`
	Unit      string     `toml:"unit,omitempty"`
	Precision int        `toml:"precision,omitempty"`
	Fill      *bool      `toml:"fill,omitempty"`
	Axis      *bool      `toml:"axis,omitempty"`
	Min       *float64   `toml:"min,omitempty"`
	Max       *float64   `toml:"max,omitempty"`
	Threshold []float64  `toml:"threshold,omitempty"`
}

// StringList lets a one-series widget use series = "cpu.total.usage" while a
// multi-series widget uses the ordinary TOML array form.
type StringList []string

func (s *StringList) UnmarshalText(text []byte) error {
	*s = StringList{string(text)}

	return nil
}

var widgetKinds = []string{
	string(WidgetLine), string(WidgetBraille), string(WidgetSparkline), string(WidgetGauge), string(WidgetText),
}

func (c Config) validatePages() error {
	seen := make(map[string]bool, len(c.Pages))

	for i, page := range c.Pages {
		path := fmt.Sprintf("page[%d]", i)
		if !pageName(page.Name) {
			return errors.Newf("%s.name %q must contain only letters, digits, '-' or '_'", path, page.Name)
		}
		if seen[page.Name] {
			return errors.Newf("%s.name %q is duplicated", path, page.Name)
		}
		seen[page.Name] = true

		if len(page.Rows) == 0 {
			return errors.Newf("%s.row must contain at least one row", path)
		}
		if err := validateRows(page.Rows, path+".row"); err != nil {
			return err
		}
	}

	return nil
}

func pageName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}

	return true
}

func validateRows(rows []Row, path string) error {
	for i, row := range rows {
		at := fmt.Sprintf("%s[%d]", path, i)
		if err := bounds(at+".height", row.Ratio, row.MinHeight, row.MaxHeight); err != nil {
			return err
		}
		if row.MinWidth < 0 {
			return errors.Newf("%s.min_width must not be negative", at)
		}
		if len(row.Children) == 0 {
			return errors.Newf("%s.child must contain at least one pane", at)
		}

		for j, pane := range row.Children {
			if err := validatePane(pane, fmt.Sprintf("%s.child[%d]", at, j)); err != nil {
				return err
			}
		}
	}

	return nil
}

func validatePane(p Pane, path string) error {
	if err := bounds(path+".width", p.Ratio, p.MinWidth, p.MaxWidth); err != nil {
		return err
	}
	if p.MinHeight < 0 {
		return errors.Newf("%s.min_height must not be negative", path)
	}

	widget, nested := p.Widget != "", len(p.Rows) > 0
	if widget == nested {
		return errors.Newf("%s must contain exactly one of widget or row", path)
	}
	if nested {
		if option := containerWidgetOption(p); option != "" {
			return errors.Newf("%s.%s is valid only for a widget", path, option)
		}

		return validateRows(p.Rows, path+".row")
	}

	return validateWidget(p, path)
}

func containerWidgetOption(p Pane) string {
	switch {
	case len(p.Series) > 0:
		return "series"
	case len(p.Tokens) > 0:
		return "tokens"
	case p.Title != "":
		return "title"
	case p.Text != "":
		return "text"
	case p.Border != "":
		return "border"
	case p.History != 0:
		return "history"
	case p.Unit != "":
		return "unit"
	case p.Precision != 0:
		return "precision"
	case p.Fill != nil:
		return "fill"
	case p.Axis != nil:
		return "axis"
	case p.Min != nil:
		return "min"
	case p.Max != nil:
		return "max"
	case len(p.Threshold) > 0:
		return "threshold"
	default:
		return ""
	}
}

func validateWidget(p Pane, path string) error {
	if err := oneOf(path+".widget", string(p.Widget), widgetKinds); err != nil {
		return err
	}
	if err := validateWidgetContent(p, path); err != nil {
		return err
	}
	if err := validateWidgetAppearance(p, path); err != nil {
		return err
	}
	if err := validateRange(p, path); err != nil {
		return err
	}
	if err := validateWidgetOptions(p, path); err != nil {
		return err
	}

	return nil
}

func validateWidgetContent(p Pane, path string) error {
	if p.Widget != WidgetText && len(p.Series) == 0 {
		return errors.Newf("%s.series must name at least one series", path)
	}
	if p.Widget == WidgetText && len(p.Series) > 0 {
		return errors.Newf("%s: a text widget cannot have series", path)
	}
	if p.Widget != WidgetText && p.Text != "" {
		return errors.Newf("%s.text is only valid for a text widget", path)
	}
	if p.Widget == WidgetGauge && len(p.Series) != 1 {
		return errors.Newf("%s: a gauge requires exactly one series", path)
	}
	if p.Widget == WidgetGauge {
		if _, err := series.ParseKey(p.Series[0]); err != nil {
			return errors.Wrapf(err, "%s.series", path)
		}
	}

	return nil
}

func validateWidgetAppearance(p Pane, path string) error {
	border := p.Border
	if border == "" {
		border = "rounded"
	}
	if err := oneOf(path+".border", border, []string{"none", "plain", "rounded", "double"}); err != nil {
		return err
	}

	unit := p.Unit
	if unit == "" {
		unit = Auto
	}
	if err := oneOf(path+".unit", unit, series.ScaleNames()); err != nil {
		return err
	}
	if p.Precision < 0 || p.Precision > 6 {
		return errors.Newf("%s.precision must be between 0 and 6", path)
	}
	if p.History < 0 {
		return errors.Newf("%s.history must not be negative", path)
	}
	if p.History > Duration(MaxSpan) {
		return errors.Newf("%s.history must not exceed %s", path, MaxSpan)
	}
	if len(p.Tokens) > 0 && len(p.Tokens) != len(p.Series) {
		return errors.Newf("%s.tokens must have one entry per series", path)
	}
	for i, name := range p.Tokens {
		if _, ok := theme.ParseToken(name); !ok {
			return errors.Newf("%s.tokens[%d] %q is not a theme token", path, i, name)
		}
	}

	return nil
}

func validateRange(p Pane, path string) error {
	if err := validateFiniteRange(p, path); err != nil {
		return err
	}
	if p.Min != nil && p.Max == nil {
		return errors.Newf("%s.min requires max", path)
	}
	if p.Max != nil {
		minimum := 0.0
		if p.Min != nil {
			minimum = *p.Min
		}
		if *p.Max <= minimum {
			return errors.Newf("%s.max must be greater than min", path)
		}
	}
	if len(p.Threshold) != 0 && (len(p.Threshold) != 2 || p.Threshold[0] >= p.Threshold[1]) {
		return errors.Newf("%s.threshold must be two increasing values", path)
	}
	if p.Widget == WidgetGauge && len(p.Threshold) == 2 {
		minimum, maximum := 0.0, 100.0
		if p.Min != nil {
			minimum = *p.Min
		}
		if p.Max != nil {
			maximum = *p.Max
		}
		if p.Threshold[0] < minimum || p.Threshold[1] > maximum {
			return errors.Newf("%s.threshold must fit between min and max", path)
		}
	}

	return nil
}

func validateFiniteRange(p Pane, path string) error {
	if p.Min != nil && !finite(*p.Min) {
		return errors.Newf("%s.min must be finite", path)
	}
	if p.Max != nil && !finite(*p.Max) {
		return errors.Newf("%s.max must be finite", path)
	}
	for i, value := range p.Threshold {
		if !finite(value) {
			return errors.Newf("%s.threshold[%d] must be finite", path, i)
		}
	}

	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validateWidgetOptions(p Pane, path string) error {
	if p.Widget == WidgetText {
		if p.History != 0 || p.Unit != "" || p.Precision != 0 || p.Fill != nil || p.Axis != nil ||
			p.Min != nil || p.Max != nil || len(p.Threshold) > 0 {
			return errors.Newf("%s: graph options are not valid for a text widget", path)
		}
	}
	switch p.Widget {
	case WidgetGauge:
		if p.History != 0 || p.Fill != nil || p.Axis != nil {
			return errors.Newf("%s: history, fill and axis are not valid for a gauge", path)
		}
	case WidgetSparkline:
		if p.Fill != nil || p.Axis != nil {
			return errors.Newf("%s: fill and axis are not valid for a sparkline", path)
		}
	}
	if p.Widget != WidgetGauge && len(p.Threshold) > 0 {
		return errors.Newf("%s.threshold is currently valid only for a gauge", path)
	}

	return nil
}

func bounds(path string, ratio, minimum, maximum int) error {
	if ratio < 0 {
		return errors.Newf("%s ratio must not be negative", path)
	}
	if minimum < 0 || maximum < 0 {
		return errors.Newf("%s bounds must not be negative", path)
	}
	if maximum > 0 && minimum > maximum {
		return errors.Newf("%s minimum %d exceeds maximum %d", path, minimum, maximum)
	}

	return nil
}

// ValidateSeries connects a pure configuration to the registry populated by
// collectors. It runs after collector construction and on every hot reload.
func (c Config) ValidateSeries(registry *series.Registry) error {
	if len(c.Pages) == 0 {
		return nil
	}
	if registry == nil {
		return errors.New("validating configured pages: no series registry")
	}

	for i, page := range c.Pages {
		if err := validateRowSeries(page.Rows, registry, fmt.Sprintf("page[%d].row", i)); err != nil {
			return err
		}
	}

	return nil
}

func validateRowSeries(rows []Row, registry *series.Registry, path string) error {
	for i, row := range rows {
		for j, pane := range row.Children {
			at := fmt.Sprintf("%s[%d].child[%d]", path, i, j)
			if len(pane.Rows) > 0 {
				if err := validateRowSeries(pane.Rows, registry, at+".row"); err != nil {
					return err
				}
				continue
			}
			if pane.Widget == WidgetText {
				continue
			}
			if err := validatePaneSeries(pane, registry, at); err != nil {
				return err
			}
		}
	}

	return nil
}

func validatePaneSeries(pane Pane, registry *series.Registry, path string) error {
	var unit series.Unit

	for i, source := range pane.Series {
		pattern, err := series.ParsePattern(source)
		if err != nil {
			return errors.Wrapf(err, "%s series %q", path, source)
		}
		candidate, ok := registry.PatternUnit(pattern)
		if !ok {
			return errors.Newf("%s series %q is unknown or has mixed units", path, source)
		}
		if i > 0 && candidate != unit {
			return errors.Newf("%s mixes %s and %s series", path, unitName(unit), unitName(candidate))
		}
		unit = candidate
	}

	scale, _ := series.ParseScale(pane.Unit)
	if !unit.SupportsScale(scale) {
		return errors.Newf("%s unit %q is not valid for %s series", path, pane.Unit, unitName(unit))
	}

	return nil
}

func unitName(unit series.Unit) string {
	if name := unit.String(); name != "" {
		return name
	}

	return "unitless"
}
