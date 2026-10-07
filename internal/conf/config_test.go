package conf_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func write(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWithNoFilesIsTheDefault(t *testing.T) {
	t.Parallel()

	got, err := conf.Sources{}.Load()
	if err != nil {
		t.Fatal(err)
	}

	if time.Duration(got.General.Interval) != time.Second {
		t.Errorf("interval = %s, want 1s", got.General.Interval)
	}

	if got.Graphs.Glyphs != conf.Auto {
		t.Errorf("glyphs = %q, want auto", got.Graphs.Glyphs)
	}
}

func TestConfigCloneOwnsNestedLayoutValues(t *testing.T) {
	t.Parallel()

	fill, minimum := true, 1.0
	original := conf.Config{
		Processes: conf.Processes{Columns: []string{"pid"}},
		Filters:   map[string]string{"all": "pid > 0"},
		Theme:     map[string]string{"cpu": "1"},
		Keys:      map[string][]string{"global.quit": {"q"}},
		Pages: []conf.Page{{Rows: []conf.Row{{Children: []conf.Pane{{
			Widget: conf.WidgetLine, Series: conf.StringList{"cpu.total.usage"},
			Threshold: []float64{50}, Fill: &fill, Min: &minimum,
		}}}}}},
	}
	cloned := original.Clone()
	cloned.Processes.Columns[0] = "rss"
	cloned.Filters["all"] = "pid == 1"
	cloned.Theme["cpu"] = "2"
	cloned.Keys["global.quit"][0] = "x"
	pane := &cloned.Pages[0].Rows[0].Children[0]
	pane.Series[0], pane.Threshold[0], *pane.Fill, *pane.Min = "mem.used", 90, false, 2

	originalPane := original.Pages[0].Rows[0].Children[0]
	if original.Processes.Columns[0] != "pid" || original.Filters["all"] != "pid > 0" ||
		original.Theme["cpu"] != "1" || original.Keys["global.quit"][0] != "q" ||
		originalPane.Series[0] != "cpu.total.usage" || originalPane.Threshold[0] != 50 ||
		!*originalPane.Fill || *originalPane.Min != 1 {
		t.Fatalf("clone mutated its source: %#v", original)
	}
}

func TestMissingFileIsSkipped(t *testing.T) {
	t.Parallel()

	sources := conf.Sources{User: filepath.Join(t.TempDir(), "nowhere.toml")}

	if _, err := sources.Load(); err != nil {
		t.Fatalf("a file that is not there should not fail the load: %v", err)
	}
}

func TestLaterLayerOverridesOnlyWhatItNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	system := filepath.Join(dir, "system.toml")
	user := filepath.Join(dir, "user.toml")

	write(t, system, `
[general]
interval = "2s"
span = "10m"

[processes]
sort = "rss"
tree = true
`)
	write(t, user, `
[general]
span = "5m"
`)

	got, err := conf.Sources{System: system, User: user}.Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := 2 * time.Second; time.Duration(got.General.Interval) != want {
		t.Errorf("interval = %s, want %s — the system layer should survive", got.General.Interval, want)
	}

	if want := 5 * time.Minute; time.Duration(got.General.Span) != want {
		t.Errorf("span = %s, want %s", got.General.Span, want)
	}

	if got.Processes.Sort != "rss" || !got.Processes.Tree {
		t.Errorf("processes = %+v, want the system layer untouched", got.Processes)
	}
}

func TestNamedFiltersAccumulateAcrossLayers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	system := filepath.Join(dir, "system.toml")
	user := filepath.Join(dir, "user.toml")

	write(t, system, "[filters]\nheavy = \"cpu > 10\"\nmine = \"user == $USER\"\n")
	write(t, user, "[filters]\nmine = \"user == $USER and cpu > 0\"\n")

	got, err := conf.Sources{System: system, User: user}.Load()
	if err != nil {
		t.Fatal(err)
	}

	if got.Filters["heavy"] != "cpu > 10" {
		t.Errorf("heavy = %q, want the system one kept", got.Filters["heavy"])
	}

	if got.Filters["mine"] != "user == $USER and cpu > 0" {
		t.Errorf("mine = %q, want the user one to win", got.Filters["mine"])
	}
}

func TestProfileIsTheLastLayer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	user := filepath.Join(dir, "config.toml")
	profile := filepath.Join(dir, "profiles", "server.toml")

	write(t, user, "[graphs]\ndevice = \"cpu\"\n")
	write(t, profile, "[graphs]\ndevice = \"network\"\n")

	got, err := conf.Sources{User: user, Profile: profile}.Load()
	if err != nil {
		t.Fatal(err)
	}

	if got.Graphs.Device != "network" {
		t.Errorf("device = %q, want the profile to win", got.Graphs.Device)
	}
}

func TestNamedProfileMustExist(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "profiles", "missing.toml")
	_, err := (conf.Sources{Profile: path}).Load()
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing profile error = %v", err)
	}
}

func TestUnknownSettingIsRefusedWithAPosition(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general]\ninterval = \"1s\"\nintrval = \"2s\"\n")

	_, err := conf.Sources{User: path}.Load()
	if err == nil {
		t.Fatal("a misspelt key should not load silently")
	}

	for _, want := range []string{"line 3", "intrval"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestBrokenSyntaxIsRefused(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general\ninterval = \"1s\"\n")

	if _, err := (conf.Sources{User: path}).Load(); err == nil {
		t.Fatal("a broken file should not load")
	}
}

func TestValidateRejectsWhatCannotBeHonoured(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an interval below the limit":    "[general]\ninterval = \"1ms\"\n",
		"an unknown contrast mode":       "[appearance]\ncontrast = \"washed-out\"\n",
		"an unknown background mode":     "[appearance]\nbackground = \"purple\"\n",
		"a span below the period":        "[general]\ninterval = \"10s\"\nspan = \"1s\"\n",
		"a span above the limit":         "[general]\nspan = \"25h\"\n",
		"an unknown glyph family":        "[graphs]\nglyphs = \"runes\"\n",
		"an unknown graph device":        "[graphs]\ndevice = \"processor\"\n",
		"an unknown sort key":            "[processes]\nsort = \"colour\"\n",
		"no log lines":                   "[logs]\nlines = 0\n",
		"too many log lines":             "[logs]\nlines = 10001\n",
		"a filter that will not compile": "[filters]\nbad = \"cpu >\"\n",
		"an unknown theme token":         "[theme]\nnope = \"1\"\n",
		"an invalid theme colour":        "[theme]\ncpu = \"not-a-colour\"\n",
	}

	for what, body := range cases {
		t.Run(what, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.toml")
			write(t, path, body)

			if _, err := (conf.Sources{User: path}).Load(); err == nil {
				t.Fatalf("%s should be refused", what)
			}
		})
	}
}

func TestFilterResolvesANameBeforeAnExpression(t *testing.T) {
	t.Parallel()

	c := conf.Default()
	c.Filters = map[string]string{"mine": "cpu > 1"}

	source, expr, err := c.Filter("mine")
	if err != nil {
		t.Fatal(err)
	}

	if source != "cpu > 1" || expr == nil {
		t.Errorf("Filter(mine) = %q, %v", source, expr)
	}

	source, _, err = c.Filter("rss > 100")
	if err != nil {
		t.Fatal(err)
	}

	if source != "rss > 100" {
		t.Errorf("an expression should pass through, got %q", source)
	}

	if _, _, err := c.Filter("cpu >"); err == nil {
		t.Error("a broken expression should fail")
	}
}

func TestCapsOverrideOnlyWhatIsNamed(t *testing.T) {
	t.Parallel()

	detected := render.Caps{Glyphs: render.Braille, Colors: render.Ansi256}

	c := conf.Default()
	if got := c.Caps(detected); got != detected {
		t.Errorf("auto should leave detection alone, got %+v", got)
	}

	c.Graphs.Glyphs = "ascii"

	got := c.Caps(detected)
	if got.Glyphs != render.ASCII || got.Colors != render.Ansi256 {
		t.Errorf("Caps = %+v, want only the glyphs replaced", got)
	}
}

func TestPaletteNamesTokens(t *testing.T) {
	t.Parallel()

	c := conf.Default()
	c.Theme = map[string]string{"cpu": "#ff0000"}

	palette := c.Palette()
	if _, ok := palette[theme.CPU]; !ok {
		t.Errorf("palette = %v, want the cpu token set", palette)
	}
}

func TestPaletteCanInheritTheTerminalForeground(t *testing.T) {
	t.Parallel()

	c := conf.Default()
	c.Theme = map[string]string{"label": "inherit"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Palette()[theme.Label].(lipgloss.NoColor); !ok {
		t.Fatalf("label colour = %#v, want the terminal default", c.Palette()[theme.Label])
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "hytop", "config.toml")

	want := conf.Default()
	want.General.Span = conf.Duration(5 * time.Minute)
	want.Processes.Sort = "rss"
	want.Filters = map[string]string{"mine": "user == $USER"}
	want.Theme = map[string]string{"cpu": "#7aa2f7"}
	want.Pages = []conf.Page{{
		Name: "notes",
		Rows: []conf.Row{{Children: []conf.Pane{{
			Widget: "text", Text: "remember this", Border: "none",
		}}}},
	}}

	if err := conf.Save(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := conf.Sources{User: path}.Load()
	if err != nil {
		t.Fatalf("hytop should be able to read back what it wrote: %v", err)
	}

	if got.General.Span != want.General.Span || got.Processes.Sort != want.Processes.Sort {
		t.Errorf("round trip lost something: %+v", got)
	}

	if got.Filters["mine"] != want.Filters["mine"] || got.Theme["cpu"] != want.Theme["cpu"] {
		t.Errorf("round trip lost a table: %+v %+v", got.Filters, got.Theme)
	}
	if len(got.Pages) != 1 || got.Pages[0].Rows[0].Children[0].Text != "remember this" {
		t.Errorf("round trip lost a page: %#v", got.Pages)
	}
}

func TestDeclarativePagesDecodeTheDocumentedShape(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, `
[[page]]
name = "overview"
title = "My machine"

	  [[page.row]]
  ratio = 2
  min_height = 6

	    [[page.row.child]]
	    widget = "line"
	    series = ["cpu.total.usage", "psi.cpu.some.avg10"]
	    tokens = ["cpu", "cpu-pressure"]
    title = "CPU"
    history = "5m"
    unit = "auto"
    precision = 1
    fill = false

    [[page.row.child]]
    ratio = 2
    widget = "braille"
    series = ["cpu.core.*.usage"]
    border = "double"

  [[page.row]]
    [[page.row.child]]
      [[page.row.child.row]]
        [[page.row.child.row.child]]
        widget = "text"
        text = "static information"
        border = "none"
`)

	got, err := (conf.Sources{User: path}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pages) != 1 || got.Pages[0].Name != "overview" || len(got.Pages[0].Rows) != 2 {
		t.Fatalf("pages = %#v", got.Pages)
	}
	first := got.Pages[0].Rows[0].Children[0]
	if len(first.Series) != 2 || first.Series[0] != "cpu.total.usage" || first.Series[1] != "psi.cpu.some.avg10" {
		t.Errorf("series decoded as %#v", first.Series)
	}
	if first.Fill == nil || *first.Fill {
		t.Errorf("fill = %v, want an explicit false", first.Fill)
	}
}

func TestConfiguredPageSeriesAreCheckedAgainstCollectors(t *testing.T) {
	t.Parallel()

	registry := &series.Registry{}
	registry.MustRegister(
		series.Def{Template: "cpu.total.usage", Unit: series.Percent},
		series.Def{Template: "cpu.core.{n}.usage", Unit: series.Percent},
		series.Def{Template: "mem.used", Unit: series.Bytes},
	)

	configured := func(keys ...string) conf.Config {
		c := conf.Default()
		c.Pages = []conf.Page{{
			Name: "custom",
			Rows: []conf.Row{{Children: []conf.Pane{{Widget: "line", Series: keys}}}},
		}}

		return c
	}

	if err := configured("cpu.core.*.usage", "cpu.total.usage").ValidateSeries(registry); err != nil {
		t.Fatalf("known compatible series were refused: %v", err)
	}
	if err := configured("cpu.unknown").ValidateSeries(registry); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("unknown series error = %v", err)
	}
	if err := configured("cpu.total.usage", "mem.used").ValidateSeries(registry); err == nil || !strings.Contains(err.Error(), "mixes") {
		t.Errorf("mixed-unit series error = %v", err)
	}

	invalidScale := configured("cpu.total.usage")
	invalidScale.Pages[0].Rows[0].Children[0].Unit = "bits"
	if err := invalidScale.ValidateSeries(registry); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("invalid unit conversion error = %v", err)
	}

	validScale := configured("mem.used")
	validScale.Pages[0].Rows[0].Children[0].Unit = "bits"
	if err := validScale.ValidateSeries(registry); err != nil {
		t.Errorf("byte-to-bit conversion was refused: %v", err)
	}
}

func TestInvalidDeclarativeLayoutsAreRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]conf.Page{
		"unsafe name": {Name: "bad/name", Rows: []conf.Row{{Children: []conf.Pane{{Widget: "text"}}}}},
		"empty row":   {Name: "empty", Rows: []conf.Row{{}}},
		"two contents": {
			Name: "both",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Widget: "text",
				Rows:   []conf.Row{{Children: []conf.Pane{{Widget: "text"}}}},
			}}}},
		},
		"widget option on a nested pane": {
			Name: "nested-option",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Title: "ignored",
				Rows:  []conf.Row{{Children: []conf.Pane{{Widget: "text"}}}},
			}}}},
		},
		"unknown widget": {Name: "bad", Rows: []conf.Row{{Children: []conf.Pane{{Widget: "clock"}}}}},
		"invalid bounds": {
			Name: "bounds",
			Rows: []conf.Row{{Children: []conf.Pane{{Widget: "text", MinWidth: 20, MaxWidth: 10}}}},
		},
		"gauge wildcard": {
			Name: "gauge",
			Rows: []conf.Row{{Children: []conf.Pane{{Widget: "gauge", Series: conf.StringList{"cpu.*"}}}}},
		},
		"ignored text option": {
			Name: "text",
			Rows: []conf.Row{{Children: []conf.Pane{{Widget: "text", History: conf.Duration(time.Minute)}}}},
		},
		"unsupported threshold": {
			Name: "line",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Widget: "line", Series: conf.StringList{"cpu.total.usage"}, Threshold: []float64{70, 90},
			}}}},
		},
		"sparkline axis": {
			Name: "spark",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Widget: "sparkline", Series: conf.StringList{"cpu.total.usage"}, Axis: boolPointer(true),
			}}}},
		},
		"nonfinite maximum": {
			Name: "maximum",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Widget: "line", Series: conf.StringList{"cpu.total.usage"}, Max: floatPointer(math.NaN()),
			}}}},
		},
		"nonfinite threshold": {
			Name: "threshold",
			Rows: []conf.Row{{Children: []conf.Pane{{
				Widget: "gauge", Series: conf.StringList{"cpu.total.usage"}, Threshold: []float64{70, math.Inf(1)},
			}}}},
		},
	}

	for name, page := range cases {
		page := page
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := conf.Default()
			c.Pages = []conf.Page{page}
			if err := c.Validate(); err == nil {
				t.Fatal("invalid page passed validation")
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

func floatPointer(value float64) *float64 { return &value }

func TestDiscoverFollowsXDG(t *testing.T) {
	t.Parallel()

	xdg := filepath.Join(string(filepath.Separator), "x")
	home := filepath.Join(string(filepath.Separator), "home", "someone")
	env := map[string]string{"XDG_CONFIG_HOME": xdg, "HOME": home}
	sources := conf.Discover("", "server", func(key string) string { return env[key] })

	if want := filepath.Join(xdg, "hytop", "config.toml"); sources.User != want {
		t.Errorf("User = %q, want %q", sources.User, want)
	}

	if want := filepath.Join(xdg, "hytop", "profiles", "server.toml"); sources.Profile != want {
		t.Errorf("Profile = %q, want %q", sources.Profile, want)
	}

	delete(env, "XDG_CONFIG_HOME")

	sources = conf.Discover("", "", func(key string) string { return env[key] })
	if want := filepath.Join(home, ".config", "hytop", "config.toml"); sources.User != want {
		t.Errorf("User = %q, want %q", sources.User, want)
	}

	if sources.Profile != "" {
		t.Errorf("Profile = %q, want none without a name", sources.Profile)
	}
}

func TestExplicitPathReplacesTheSearch(t *testing.T) {
	t.Parallel()

	sources := conf.Discover("/tmp/one.toml", "", func(string) string { return "" })

	if sources.System != "" || sources.User != "" {
		t.Errorf("-c should stand alone, got %+v", sources)
	}

	if got := sources.Files(); len(got) != 1 || got[0] != "/tmp/one.toml" {
		t.Errorf("Files = %v", got)
	}
}
