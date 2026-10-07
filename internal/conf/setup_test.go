package conf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/conf"
)

func TestNeedsSetupOnlyWithoutAnyConfigurationLayer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sources := conf.Sources{
		System: filepath.Join(dir, "etc", "config.toml"),
		User:   filepath.Join(dir, "user", "config.toml"),
	}

	needed, err := conf.NeedsSetup(sources)
	if err != nil || !needed {
		t.Fatalf("NeedsSetup() = %v, %v, want true", needed, err)
	}

	write(t, sources.System, "")
	needed, err = conf.NeedsSetup(sources)
	if err != nil || needed {
		t.Fatalf("NeedsSetup() with a system layer = %v, %v, want false", needed, err)
	}

	needed, err = conf.NeedsSetup(conf.Sources{User: sources.User, Extra: "chosen.toml"})
	if err != nil || needed {
		t.Fatalf("NeedsSetup() with an explicit layer = %v, %v, want false", needed, err)
	}
}

func TestSaveSetupWritesOnlyChoicesThatChanged(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "hytop", "config.toml")
	base := conf.Default()
	selected := base.Clone()
	selected.Appearance.Contrast = conf.ContrastHigh
	selected.Graphs.Glyphs = "block"
	selected.General.Span = conf.Duration(5 * time.Minute)
	selected.Processes.Tree = true

	if err := conf.SaveSetup(path, base, selected); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, want := range []string{"configured by hytop setup", "contrast", "glyphs", "span", "tree"} {
		if !strings.Contains(text, want) {
			t.Errorf("initial configuration does not contain %q:\n%s", want, text)
		}
	}
	for _, frozenDefault := range []string{"device", "descending", "columns", "lines"} {
		if strings.Contains(text, frozenDefault) {
			t.Errorf("initial configuration froze default %q:\n%s", frozenDefault, text)
		}
	}

	got, err := (conf.Sources{User: path}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Appearance.Contrast != conf.ContrastHigh || got.Graphs.Glyphs != "block" ||
		got.General.Span != selected.General.Span || !got.Processes.Tree {
		t.Errorf("loaded setup = %+v, want the selected values", got)
	}
}

func TestSaveSetupMarksRecommendedDefaultsWithoutReplacingAFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	base := conf.Default()
	if err := conf.SaveSetup(path, base, base); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "configured by hytop setup") {
		t.Fatalf("default setup did not leave a completion marker: %q", contents)
	}

	if err := conf.SaveSetup(path, base, base); err == nil {
		t.Fatal("SaveSetup replaced an existing configuration")
	}
}
