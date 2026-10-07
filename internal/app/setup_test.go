package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

func setupProgram(wizard *setup, width, height int) *reactea.App {
	program := reactea.New(wizard, reactea.WithSize(width, height))
	program.Start()

	return program
}

func TestSetupEditsPreviewsAndSavesTheSelectedValues(t *testing.T) {
	t.Parallel()

	base := conf.Default()
	var saved conf.Config
	wizard := newSetup(base, render.Caps{Glyphs: render.Braille, Colors: render.Ansi256},
		"/tmp/config.toml", func(selected conf.Config) error {
			saved = selected

			return nil
		})
	program := setupProgram(wizard, 80, 24)

	view := testkit.Plain(program)
	for _, want := range []string{"hytop setup", "Text contrast", "Preview", "Tasks", "defaults"} {
		if !strings.Contains(view, want) {
			t.Fatalf("setup is missing %q:\n%s", want, view)
		}
	}
	for _, hidden := range []string{"Terminal background", "Graph style", "Collection period", "Process tree"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("setup shows the later question %q too early:\n%s", hidden, view)
		}
	}

	testkit.SendKeys(program,
		"right", "enter",
		"right", "enter",
		"right", "enter",
		"right", "enter",
		"enter",
	)

	if !wizard.complete {
		t.Fatal("setup did not complete after saving")
	}
	if saved.Appearance.Contrast != conf.ContrastStandard || saved.Appearance.Background != conf.Auto {
		t.Errorf("appearance = %+v, want standard contrast with automatic background detection", saved.Appearance)
	}
	if saved.Graphs.Glyphs != "braille" || saved.Graphs.Colors != conf.Auto {
		t.Errorf("graphs = %+v, want explicit glyphs and automatic colours", saved.Graphs)
	}
	if saved.General.Span != conf.Duration(time.Minute) || saved.General.Interval != conf.Duration(2*time.Second) {
		t.Errorf("general = %+v, want the chosen collection period", saved.General)
	}
	if !saved.Processes.Tree || saved.Processes.Kernel {
		t.Errorf("processes = %+v, want only the tree enabled", saved.Processes)
	}
}

func TestSetupKeepsTheReviewOpenWhenSavingFails(t *testing.T) {
	t.Parallel()

	wizard := newSetup(conf.Default(), render.Caps{Glyphs: render.ASCII, Colors: render.Mono},
		"/read-only/config.toml", func(conf.Config) error { return errors.New("disk is read-only") })
	program := setupProgram(wizard, 72, 22)

	testkit.SendKeys(program, "r", "enter")

	if wizard.complete {
		t.Fatal("setup completed despite the save error")
	}
	if got := testkit.Plain(program); !strings.Contains(got, "disk is read-only") {
		t.Fatalf("save failure is not visible:\n%s", got)
	}
}

func TestSetupExplainsWhenTheTerminalIsTooSmall(t *testing.T) {
	t.Parallel()

	wizard := newSetup(conf.Default(), render.Caps{Glyphs: render.ASCII, Colors: render.Mono}, "config.toml", nil)
	program := setupProgram(wizard, 24, 6)

	if got := testkit.Plain(program); !strings.Contains(got, "resize to at least 28×7") {
		t.Fatalf("small setup screen does not explain its minimum size:\n%s", got)
	}
}
