package sysread_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect/sysread"
)

func TestStringAndFloat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attribute")
	if err := os.WriteFile(path, []byte(" 12.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, ok := sysread.String(path); !ok || got != "12.5" {
		t.Fatalf("String() = %q, %v", got, ok)
	}
	if got, ok := sysread.Float(path); !ok || got != 12.5 {
		t.Fatalf("Float() = %v, %v", got, ok)
	}
}

func TestSensorSourceIdentifiesSymlinkedChannels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	target, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(target, "power1")
	if got := sysread.SensorSource(alias, "power1"); got != want {
		t.Fatalf("sensor source = %q, want %q", got, want)
	}
	for _, dir := range []string{"", filepath.Join(root, "missing")} {
		if got := sysread.SensorSource(dir, "power1"); got != "" {
			t.Fatalf("unresolved directory %q returned source %q", dir, got)
		}
	}
}
