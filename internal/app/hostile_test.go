package app_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
)

func TestNothingOnScreenCanRewriteTheScreen(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Units = func() []unitmodel.Unit {
			return []unitmodel.Unit{
				{
					Name: "evil.service", Load: "loaded", Active: "active", Sub: "running",
					Description: "ok\x1b[2J\x1b[HWIPED\rOVER\x1b]0;stolen\x07",
				},
			}
		}
	})

	mode(t, program, "Services")

	got := program.View().Content

	for _, sequence := range []struct {
		what string
		text string
	}{
		{"a clear-screen sequence", "\x1b[2J"},
		{"a cursor-home sequence", "\x1b[H"},
		{"a window-title sequence", "\x1b]0;"},
		{"a carriage return", "\r"},
	} {
		if strings.Contains(got, sequence.text) {
			t.Errorf("%s from a unit description reached the frame", sequence.what)
		}
	}

	if !strings.Contains(plain(program), "evil.service") {
		t.Error("the hostile unit was dropped from the list rather than made safe")
	}
}

func TestSanitisingNeverManglesTheStylingItself(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, nil)

	for _, available := range page.Modes() {
		mode(t, program, available.Title)

		if got := program.View().Content; strings.Contains(got, "?[") {
			t.Errorf("%s has a broken escape on screen — styling went through the sanitiser:\n%s",
				available.Title, got)
		}
	}
}

func TestAProcessCannotRewriteTheScreenThroughItsCommandLine(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixtureWith(t, nil)

	memory.WriteProcesses([]procmodel.Process{
		{PID: 1, PPID: 0, Name: "init", User: "root", Cmdline: "/sbin/init", Threads: 1},
		{
			PID: 99, PPID: 1, User: "hayao", Threads: 1, CPU: 50,
			Name: "evil\x1b[2J", Cmdline: "evil\x1b[2J\x1b[HWIPED\rOVER",
		},
	})

	mode(t, program, "Processes")

	if got := program.View().Content; strings.Contains(got, "\x1b[2J") || strings.Contains(got, "\r") {
		t.Error("a process rewrote the screen through its own command line")
	}
}

func TestContainerAndInterfaceNamesCannotRewriteTheScreen(t *testing.T) {
	t.Parallel()

	const hostile = "evil\x1b[2J\rOVER"
	program, memory, _ := fixtureWith(t, func(env *app.Env) {
		env.Interfaces = func() []string { return []string{hostile} }
	})
	memory.WriteProcesses([]procmodel.Process{{
		PID: 1, Name: "task", Container: hostile, Runtime: "docker", RSS: 1,
	}})

	mode(t, program, "Containers")
	if got := program.View().Content; strings.Contains(got, "\x1b[2J") || strings.Contains(got, "\r") {
		t.Error("a container name reached the terminal as control input")
	}

	mode(t, program, "Graphs")
	for range 3 {
		press(t, program, "j")
	}
	if got := program.View().Content; strings.Contains(got, "\x1b[2J") || strings.Contains(got, "\r") {
		t.Error("an interface name reached the terminal as control input")
	}
}

func TestRebindingAProcessTableKeyTakesEffect(t *testing.T) {
	t.Parallel()

	for _, action := range []keymap.Action{
		keymap.Down, keymap.Up, keymap.SortCPU, keymap.SortMem, keymap.SortPID, keymap.SortName,
	} {
		name := keymap.Name(keymap.Processes, action)

		keys, err := keymap.Default().Rebind(map[string][]string{name: {"Y"}})
		if err != nil {
			t.Fatalf("rebinding %s: %v", name, err)
		}

		if bound := keys.Keys(keymap.Processes, action); len(bound) != 1 || bound[0] != "Y" {
			t.Errorf("%s is bound to %v after a rebind, want [Y]", name, bound)
		}
	}
}

func TestTheProcessTableObeysARebind(t *testing.T) {
	t.Parallel()

	program, _, view := fixtureWith(t, func(env *app.Env) {
		settings := env.Config
		settings.Keys = map[string][]string{"processes.sort-mem": {"Y"}}
		env.Config = settings
	})

	mode(t, program, "Processes")

	press(t, program, "Y")

	if by, _ := view.Sort(); by != store.SortMem {
		t.Errorf("the rebound sort key did nothing: sorting by %s", by)
	}
}
