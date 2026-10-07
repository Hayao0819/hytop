package conf_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestWatcherKeepsTheLastGoodConfiguration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general]\nspan = \"5m\"\n")

	watcher, err := conf.NewWatcher(conf.Sources{User: path}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if want := 5 * time.Minute; time.Duration(watcher.Config().General.Span) != want {
		t.Fatalf("span = %s, want %s", watcher.Config().General.Span, want)
	}

	write(t, path, "[general\nspan = \"9m\"\n")
	watcher.Reload()

	if want := 5 * time.Minute; time.Duration(watcher.Config().General.Span) != want {
		t.Errorf("a broken file replaced the running config with %s", watcher.Config().General.Span)
	}

	if watcher.Problem() == "" {
		t.Error("a broken file should say so")
	}

	write(t, path, "[general]\nspan = \"9m\"\n")
	watcher.Reload()

	if want := 9 * time.Minute; time.Duration(watcher.Config().General.Span) != want {
		t.Errorf("span = %s, want %s once the file parses again", watcher.Config().General.Span, want)
	}

	if watcher.Problem() != "" {
		t.Errorf("the problem should clear, got %q", watcher.Problem())
	}
}

func TestWatcherRetainsAConfigRejectedByRuntimeValidation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general]\nspan = \"5m\"\n")

	watcher, err := conf.NewWatcher(conf.Sources{User: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry := &series.Registry{}
	registry.MustRegister(series.Def{Template: "cpu.total.usage", Unit: series.Percent})
	if err := watcher.AddValidator(func(c conf.Config) error { return c.ValidateSeries(registry) }); err != nil {
		t.Fatal(err)
	}

	write(t, path, `
[[page]]
name = "broken"
  [[page.row]]
    [[page.row.child]]
    widget = "line"
    series = "cpu.typo.usage"
`)
	watcher.Reload()

	if len(watcher.Config().Pages) != 0 {
		t.Errorf("runtime-invalid pages replaced the last good configuration: %#v", watcher.Config().Pages)
	}
	if !strings.Contains(watcher.Problem(), "cpu.typo.usage") {
		t.Errorf("problem = %q, want the bad series", watcher.Problem())
	}
}

func TestWatcherStartupFailureIsFatal(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general\n")

	if _, err := conf.NewWatcher(conf.Sources{User: path}, nil); err == nil {
		t.Error("there is no last-good configuration at startup, so this must fail")
	}
}

func TestWatcherReturnsDetachedConfigurations(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, `
[filters]
workers = "cpu > 1"
[keys]
"global.quit" = ["x"]
[[page]]
name = "custom"
  [[page.row]]
    [[page.row.child]]
    widget = "gauge"
    series = ["cpu.total.usage"]
    threshold = [50, 90]
`)

	watcher, err := conf.NewWatcher(conf.Sources{User: path}, nil)
	if err != nil {
		t.Fatal(err)
	}

	first := watcher.Config()
	first.Filters["workers"] = "pid == 1"
	first.Keys["global.quit"][0] = "q"
	first.Pages[0].Rows[0].Children[0].Series[0] = "mem.used"
	first.Pages[0].Rows[0].Children[0].Threshold[0] = 1

	second := watcher.Config()
	if second.Filters["workers"] != "cpu > 1" || second.Keys["global.quit"][0] != "x" {
		t.Fatalf("maps alias watcher state: %#v %#v", second.Filters, second.Keys)
	}
	pane := second.Pages[0].Rows[0].Children[0]
	if pane.Series[0] != "cpu.total.usage" || pane.Threshold[0] != 50 {
		t.Fatalf("layout aliases watcher state: %#v", pane)
	}
}

func TestWatcherReloadsAfterAWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	write(t, path, "[general]\nspan = \"5m\"\n")

	watcher, err := conf.NewWatcher(conf.Sources{User: path}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()

	got := waitForReload(t, watcher, func() {
		write(t, path, "[general]\nspan = \"9m\"\n")
	})
	if want := 9 * time.Minute; time.Duration(got.General.Span) != want {
		t.Fatalf("span = %s, want %s", got.General.Span, want)
	}
}

func TestWatcherNoticesAConfigurationDirectoryCreatedLater(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "new", "hytop", "config.toml")
	watcher, err := conf.NewWatcher(conf.Sources{User: path}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = watcher.Run(ctx) }()

	got := waitForReload(t, watcher, func() {
		write(t, path, "[general]\nspan = \"9m\"\n")
		write(t, filepath.Join(root, ".wake"), time.Now().String())
	})
	if want := 9 * time.Minute; time.Duration(got.General.Span) != want {
		t.Fatalf("span = %s, want %s", got.General.Span, want)
	}
}

func waitForReload(t *testing.T, watcher *conf.Watcher, stimulate func()) conf.Config {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stimulate()
		if watcher.Generation() > 0 {
			return watcher.Config()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("configuration was not reloaded")

	return conf.Config{}
}
