//go:build linux

package diskusage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

func TestSomethingThatIsNotAPlainFileStillCounts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	deep := filepath.Join(root, "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(deep, "real"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("real", filepath.Join(deep, "link")); err != nil {
		t.Fatal(err)
	}

	scanner := diskusage.NewScanner(func() ([]diskmodel.Mount, error) { return nil, nil })

	scan := settled(t, scanner, func() { scanner.Start(context.Background(), root) })

	got := entry(t, scan, "sub")
	if got.Items != 2 {
		t.Errorf("sub holds %d items, want 2 — the file and the symlink", got.Items)
	}
}

func TestTheRootAndTheWalkBelowItCountAlike(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := os.Symlink("nowhere", filepath.Join(root, "top")); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("nowhere", filepath.Join(deep, "below")); err != nil {
		t.Fatal(err)
	}

	scanner := diskusage.NewScanner(func() ([]diskmodel.Mount, error) { return nil, nil })

	scan := settled(t, scanner, func() { scanner.Start(context.Background(), root) })

	var (
		top   = entry(t, scan, "top")
		below = entry(t, scan, "sub")
	)

	if top.Items != below.Items {
		t.Errorf("the same symlink counts %d at the root and %d one level down",
			top.Items, below.Items)
	}
}
