package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func dashboardConfig(t *testing.T, title string) (conf.Config, *series.Registry) {
	t.Helper()

	maximum := 100.0
	c := settings(t)
	c.Pages = []conf.Page{{
		Name:  "lab",
		Title: title,
		Rows: []conf.Row{
			{
				Ratio: 2,
				Children: []conf.Pane{
					{
						Widget: "braille", Title: "Per core", Ratio: 3,
						Series: conf.StringList{"cpu.core.*.usage"},
					},
					{Widget: "text", Title: "Notes", Text: "configured without Go code", Ratio: 2},
				},
			},
			{
				Children: []conf.Pane{{
					Widget: "gauge", Title: "CPU gauge", Series: conf.StringList{"cpu.total.usage"},
					Max: &maximum, Threshold: []float64{70, 90},
				}},
			},
		},
	}}

	registry := &series.Registry{}
	registry.MustRegister(
		series.Def{Template: "cpu.core.{n}.usage", Unit: series.Percent},
		series.Def{Template: "cpu.total.usage", Unit: series.Percent},
	)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := c.ValidateSeries(registry); err != nil {
		t.Fatal(err)
	}

	return c, registry
}

func enterDashboard(t *testing.T, program *reactea.App, pressTab func()) {
	t.Helper()

	for range 20 {
		if program.Route() == "/dashboards/lab" {
			return
		}
		pressTab()
	}

	t.Fatalf("custom dashboard was not reachable; stopped at %s", program.Route())
}

// reload excludes the recurring timer command from the synchronous harness.
func reload(program *reactea.App) {
	_, command := program.Update(app.Refresh(time.Now()))
	if command == nil {
		return
	}

	message := command()
	batch, ok := message.(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		return
	}
	runCommands(program, batch[len(batch)-1]())
}

func runCommands(program *reactea.App, message tea.Msg) {
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, command := range batch {
			if command != nil {
				runCommands(program, command())
			}
		}

		return
	}
	if message != nil {
		program.Send(message)
	}
}

func TestAConfiguredDashboardBecomesAWorkingPage(t *testing.T) {
	t.Parallel()

	c, registry := dashboardConfig(t, "Lab")
	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = c
		env.Registry = registry
	})

	enterDashboard(t, program, func() { press(t, program, "tab") })

	got := plain(program)
	for _, want := range []string{"Lab", "Per core", "Notes", "configured without Go code", "CPU gauge", "▲ 87 %"} {
		if !strings.Contains(got, want) {
			t.Errorf("configured page is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "? Lab") {
		t.Errorf("an unnumbered dashboard was presented as a broken key binding:\n%s", got)
	}
	if !strings.ContainsAny(got, "⣿⣾⣴⣠⢀") {
		t.Errorf("the wildcard series did not draw a graph:\n%s", got)
	}
}

func TestAConfiguredDashboardIsRebuiltInPlace(t *testing.T) {
	t.Parallel()

	c, registry := dashboardConfig(t, "Before")
	source := &live{config: c}
	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = c
		env.Live = source
		env.Registry = registry
	})
	enterDashboard(t, program, func() { press(t, program, "tab") })

	next := source.Config()
	next.Pages[0].Title = "After"
	next.Pages[0].Rows[0].Children[1].Text = "hot reloaded"
	source.set(next)
	reload(program)

	if program.Route() != "/dashboards/lab" {
		t.Fatalf("reload moved from the custom page to %s", program.Route())
	}
	got := plain(program)
	if !strings.Contains(got, "After") || !strings.Contains(got, "hot reloaded") || strings.Contains(got, "Before") {
		t.Errorf("the page did not rebuild from the new config:\n%s", got)
	}
}

func TestRemovingTheCurrentDashboardReturnsToGraphs(t *testing.T) {
	t.Parallel()

	c, registry := dashboardConfig(t, "Temporary")
	source := &live{config: c}
	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = c
		env.Live = source
		env.Registry = registry
	})
	enterDashboard(t, program, func() { press(t, program, "tab") })

	next := source.Config()
	next.Pages = nil
	source.set(next)
	reload(program)

	if program.Route() != "/graphs/cpu" {
		t.Errorf("route after removing the current page = %s, want /graphs/cpu", program.Route())
	}
}
