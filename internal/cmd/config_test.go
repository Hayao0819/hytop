package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestConfigWriteNeedsAUserDirectoryOrExplicitPath(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	root := New("test")
	root.SetArgs([]string{"config", "write"})
	root.SetOut(new(bytes.Buffer))
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "user configuration directory") {
		t.Fatalf("config write error = %v", err)
	}
}

func TestConfigWriteCreatesANewExplicitFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "new", "config.toml")
	root := New("test")
	root.SetArgs([]string{"--config", path, "--span", "7m", "config", "write"})
	root.SetOut(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `span = '7m'`) && !strings.Contains(string(contents), `span = "7m"`) {
		t.Fatalf("written config did not include overrides:\n%s", contents)
	}
}

func TestConfigShowRejectsAMissingProfile(t *testing.T) {
	t.Parallel()

	config := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	root := New("test")
	root.SetArgs([]string{"--config", config, "--profile", "missing", "config", "show"})
	root.SetOut(new(bytes.Buffer))
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "missing.toml") {
		t.Fatalf("missing profile error = %v", err)
	}
}

func TestConfigCommandsRejectUnknownDashboardSeries(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	body := `
[[page]]
name = "bad"
  [[page.row]]
    [[page.row.child]]
    widget = "line"
    series = "not.a.metric"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"show", "write"} {
		root := New("test")
		root.SetArgs([]string{"--config", path, "config", action})
		root.SetOut(new(bytes.Buffer))
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "not.a.metric") {
			t.Errorf("config %s error = %v, want the unknown series", action, err)
		}
	}
}

func TestConfigCommandsDoNotProbeRuntimeCollectors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	body := `
[[page]]
name = "portable"
  [[page.row]]
    [[page.row.child]]
    widget = "line"
    series = "psi.cpu.some.avg10"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	root := New("test")
	root.SetArgs([]string{
		"--config", path,
		"--proc", filepath.Join(t.TempDir(), "missing-proc"),
		"--sys", filepath.Join(t.TempDir(), "missing-sys"),
		"config", "show",
	})
	root.SetOut(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("config show probed runtime collectors: %v", err)
	}
}

func TestSeriesRegistryIncludesUnavailablePlatformFamilies(t *testing.T) {
	t.Parallel()

	registry, err := seriesRegistry()
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"gpu.0.util", "thermal.cpu.temp", "battery.0.capacity", "psi.cpu.some.avg10", "units.total"} {
		if _, _, ok := registry.Lookup(series.Key(key)); !ok {
			t.Errorf("series registry does not know %q", key)
		}
	}
}
