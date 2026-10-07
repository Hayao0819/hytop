package app_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/filter"
)

const unicodeGraphGlyphs = "⣿⣾⣴⣠⢀▁▂▃▄▅▆▇█"

type live struct {
	mu         sync.Mutex
	config     conf.Config
	generation uint64
	problem    string
}

func (l *live) Config() conf.Config {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.config
}

func (l *live) Generation() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.generation
}

func (l *live) Problem() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.problem
}

func (l *live) set(c conf.Config) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.config, l.generation = c, l.generation+1
}

func (l *live) fail(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.problem = reason
}

func settings(t *testing.T) conf.Config {
	t.Helper()

	c := conf.Default()
	c.General.Interval = conf.Duration(conf.MinInterval)

	return c
}

func TestAReloadAppliesProcessDefaultsWithoutErasingUnchangedInteractiveState(t *testing.T) {
	t.Parallel()

	source := &live{config: settings(t)}
	program, _, view := fixtureWith(t, func(env *app.Env) {
		env.Config = source.config
		env.Live = source
	})

	view.SetTree(true)
	next := source.Config()
	next.Processes.Sort = "rss"
	next.Processes.Descending = false
	next.Processes.Kernel = true
	next.Processes.Filter = `user == "root"`
	source.set(next)
	tick(program)

	if by, descending := view.Sort(); by != "rss" || descending {
		t.Errorf("sort after reload = %q, %v", by, descending)
	}
	if !view.Kernel() {
		t.Error("reloaded kernel-thread setting was not applied")
	}
	if filter, _ := view.Filter(); filter != `user == "root"` {
		t.Errorf("filter after reload = %q", filter)
	}
	if !view.Tree() {
		t.Error("an unchanged setting erased the interactive tree state")
	}
}

func TestReloadingAnUnusedNamedFilterKeepsTheInteractiveFilter(t *testing.T) {
	t.Parallel()

	source := &live{config: settings(t)}
	program, _, view := fixtureWith(t, func(env *app.Env) {
		env.Config = source.config
		env.Live = source
	})

	const interactive = `user == "hayao"`
	expr, err := filter.Compile(interactive)
	if err != nil {
		t.Fatal(err)
	}
	view.SetFilter(interactive, expr)

	next := source.Config()
	next.Filters = map[string]string{"unused": "cpu > 10"}
	source.set(next)
	tick(program)

	if got, _ := view.Filter(); got != interactive {
		t.Errorf("unrelated filter reload replaced %q with %q", interactive, got)
	}
}

func TestGlyphsFromTheConfigurationReachTheGraph(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Graphs.Glyphs = "ascii"

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = c })

	if got := plain(program); strings.ContainsAny(got, unicodeGraphGlyphs) {
		t.Errorf("ascii was asked for but a Unicode graph was drawn:\n%s", got)
	}
}

func TestAReloadRebuildsThePages(t *testing.T) {
	t.Parallel()

	source := &live{config: settings(t)}

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = source.config
		env.Live = source
	})

	if got := plain(program); !strings.ContainsAny(got, "⣿⣾⣴⣠⢀") {
		t.Fatalf("the fixture should start on braille:\n%s", got)
	}

	next := source.Config()
	next.Graphs.Glyphs = "ascii"
	source.set(next)

	tick(program)

	if got := plain(program); strings.ContainsAny(got, unicodeGraphGlyphs) {
		t.Errorf("the reloaded glyph setting did not take:\n%s", got)
	}
}

func TestAConfigurationThatWillNotLoadIsSaidSo(t *testing.T) {
	t.Parallel()

	source := &live{config: settings(t)}
	source.fail("config: line 3 column 1: unknown setting \"general.intrval\" — keeping the last good one")

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = source.config
		env.Live = source
	})

	if got := plain(program); !strings.Contains(got, "unknown setting") {
		t.Errorf("the key line should carry the reason:\n%s", got)
	}
}

func TestTheSettingsScreenEditsTheConfiguration(t *testing.T) {
	t.Parallel()

	var written conf.Config

	program, _, view := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.SavePath = "/tmp/hytop-test.toml"
		env.Save = func(c conf.Config) error {
			written = c

			return nil
		}
	})

	mode(t, program, "Settings")

	if got := plain(program); !strings.Contains(got, "Graph history") {
		t.Fatalf("the settings screen did not open:\n%s", got)
	}

	for range 7 {
		press(t, program, "j")
	}

	press(t, program, "l")

	if !view.Tree() {
		t.Error("turning the tree on did not reach the process list")
	}

	if got := plain(program); !strings.Contains(got, "Process tree") || !strings.Contains(got, "on") {
		t.Errorf("the screen did not follow the change:\n%s", got)
	}

	press(t, program, "w")

	if !written.Processes.Tree {
		t.Errorf("w wrote %+v, which does not carry the change", written.Processes)
	}

	if got := plain(program); !strings.Contains(got, "written to /tmp/hytop-test.toml") {
		t.Errorf("writing should say where it went:\n%s", got)
	}
}

func TestTheWriteKeyIsHiddenWithNowhereToWrite(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = settings(t) })

	mode(t, program, "Settings")

	if got := plain(program); strings.Contains(got, "w write") {
		t.Errorf("the key line offers a write with no path:\n%s", got)
	}
}

func TestTheCursorSurvivesAChange(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = settings(t) })

	mode(t, program, "Settings")
	press(t, program, "j", "j", "j", "j", "l")

	press(t, program, "l")

	if got := plain(program); !strings.Contains(got, "block") {
		t.Errorf("the cursor did not stay on the glyph row:\n%s", got)
	}
}

func TestSettingsScrollToKeepTheCursorVisible(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = settings(t) })
	mode(t, program, "Settings")
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 10})

	for range 8 {
		press(t, program, "j")
	}

	got := plain(program)
	if !strings.Contains(got, "Kernel threads") || !strings.Contains(got, "↑") {
		t.Errorf("the settings cursor moved outside the visible window:\n%s", got)
	}
}

func tick(program *reactea.App) {
	program.Update(app.Refresh(time.Now()))
}

func TestKeysFromTheConfigurationReplaceTheDefaults(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Keys = map[string][]string{"processes.toggle-tree": {"T"}}

	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}

	program, _, view := fixtureWith(t, func(env *app.Env) { env.Config = c })

	mode(t, program, "Processes")
	press(t, program, "t")

	if view.Tree() {
		t.Error("t was rebound away and still turned the tree on")
	}

	press(t, program, "T")

	if !view.Tree() {
		t.Error("the rebound key did nothing")
	}

	if got := plain(program); !strings.Contains(got, "T tree") {
		t.Errorf("the key line still advertises the old key:\n%s", got)
	}
}

func TestABindingThatCannotBeHonouredIsRefused(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Keys = map[string][]string{"processes.filter": {"t"}}

	if err := c.Validate(); err == nil {
		t.Error("two actions on one key in one scope should be refused")
	}

	c.Keys = map[string][]string{"processes.explode": {"x"}}

	if err := c.Validate(); err == nil {
		t.Error("an unknown binding should be refused")
	}
}

func TestTheHelpIsGeneratedFromTheKeymap(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Keys = map[string][]string{"global.quit": {"Q"}}

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = c })

	press(t, program, "?")

	got := plain(program)
	if !strings.Contains(got, "Q") || !strings.Contains(got, "quit") {
		t.Errorf("the help does not describe the rebound quit:\n%s", got)
	}

	for _, want := range []string{"Everywhere", "Graphs", "Processes"} {
		if !strings.Contains(got, want) {
			t.Errorf("the help is missing the %q section:\n%s", want, got)
		}
	}

	for range 200 {
		press(t, program, "j")
	}

	if got := plain(program); !strings.Contains(got, "About") {
		t.Errorf("scrolling did not reach the end of the help:\n%s", got)
	}
}
