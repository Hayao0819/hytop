package app

import (
	"time"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

const setupOmit = -1

const (
	setupContrast = iota
	setupGlyphs
	setupInterval
	setupTree
	setupReview
)

type preference struct {
	Step    int
	Group   string
	Label   string
	What    string
	Options []string
	Value   func(conf.Config) string
	Apply   func(*conf.Config, string)
}

func preferences() []preference {
	return []preference{
		{
			Step: setupOmit, Group: "Graphs", Label: "Graph history",
			What: "how far back a plot reaches", Options: []string{"30s", "1m", "5m", "15m"},
			Value: func(c conf.Config) string { return c.General.Span.String() },
			Apply: func(c *conf.Config, value string) { c.General.Span = duration(value) },
		},
		{
			Step: setupInterval, Group: "Graphs", Label: "Collection period",
			What: "how often every counter is read", Options: []string{"500ms", "1s", "2s", "5s"},
			Value: func(c conf.Config) string { return c.General.Interval.String() },
			Apply: func(c *conf.Config, value string) { c.General.Interval = duration(value) },
		},
		{
			Step: setupContrast, Group: "Appearance", Label: "Text contrast",
			What:    "auto uses the terminal foreground; standard uses muted grey",
			Options: []string{conf.Auto, conf.ContrastStandard, conf.ContrastHigh},
			Value:   func(c conf.Config) string { return c.Appearance.Contrast },
			Apply:   func(c *conf.Config, value string) { c.Appearance.Contrast = value },
		},
		{
			Step: setupOmit, Group: "Appearance", Label: "Terminal background",
			What:    "auto asks the terminal whether its background is dark or light",
			Options: []string{conf.Auto, conf.BackgroundDark, conf.BackgroundLight},
			Value:   func(c conf.Config) string { return c.Appearance.Background },
			Apply:   func(c *conf.Config, value string) { c.Appearance.Background = value },
		},
		{
			Step: setupGlyphs, Group: "Appearance", Label: "Graph style",
			What:    "braille is finer, block and ASCII are more widely supported",
			Options: append([]string{conf.Auto}, render.GlyphNames()...),
			Value:   func(c conf.Config) string { return c.Graphs.Glyphs },
			Apply:   func(c *conf.Config, value string) { c.Graphs.Glyphs = value },
		},
		{
			Step: setupOmit, Group: "Appearance", Label: "Colour depth",
			What:    "auto follows the terminal; mono disables colour",
			Options: append([]string{conf.Auto}, render.ColorNames()...),
			Value:   func(c conf.Config) string { return c.Graphs.Colors },
			Apply:   func(c *conf.Config, value string) { c.Graphs.Colors = value },
		},
		{
			Step: setupOmit, Group: "Processes", Label: "Default sort",
			What: "which column the list opens on", Options: []string{"cpu", "rss", "pid", "name"},
			Value: func(c conf.Config) string { return c.Processes.Sort },
			Apply: func(c *conf.Config, value string) { c.Processes.Sort = value },
		},
		{
			Step: setupTree, Group: "Processes", Label: "Process tree",
			What: "draw the parent links in the process list", Options: []string{"off", "on"},
			Value: func(c conf.Config) string { return boolName(c.Processes.Tree) },
			Apply: func(c *conf.Config, value string) { c.Processes.Tree = value == "on" },
		},
		{
			Step: setupOmit, Group: "Processes", Label: "Kernel threads",
			What: "show kernel workers drawn in brackets", Options: []string{"off", "on"},
			Value: func(c conf.Config) string { return boolName(c.Processes.Kernel) },
			Apply: func(c *conf.Config, value string) { c.Processes.Kernel = value == "on" },
		},
	}
}

func (r *Root) settingItems() []*page.Setting {
	edit := func(change func(*conf.Config, string)) func(string) {
		return func(value string) {
			next := r.config
			change(&next, value)

			if err := next.Validate(); err == nil {
				r.apply(next)
			}
		}
	}

	items := make([]*page.Setting, 0, len(preferences()))
	for _, spec := range preferences() {
		items = append(items, &page.Setting{
			Group: spec.Group, Label: spec.Label, What: spec.What,
			Options: spec.Options, Chosen: index(spec.Options, spec.Value(r.config)),
			Apply: edit(spec.Apply),
		})
	}

	return items
}

func index(options []string, value string) int {
	for i, option := range options {
		if option == value {
			return i
		}
	}

	return 0
}

func boolName(on bool) string {
	if on {
		return "on"
	}

	return "off"
}

// duration is only ever handed a literal from the options above, so a parse
// failure would be a typo in this file.
func duration(value string) conf.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return conf.Duration(time.Minute)
	}

	return conf.Duration(parsed)
}
