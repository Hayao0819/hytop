//go:build linux

package diskusage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

func scanned(t *testing.T, root string, mounts []diskmodel.Mount) *diskusage.Scanner {
	t.Helper()

	scanner := diskusage.NewScanner(func() ([]diskmodel.Mount, error) { return mounts, nil })

	settled(t, scanner, func() { scanner.Start(context.Background(), root) })

	return scanner
}

func TestDeletingRefusesAPathHoldingAnotherFilesystem(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	for _, dir := range []string{"keep", "keep/mounted"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	elsewhere := filepath.Join(root, "keep", "mounted", "important.txt")
	if err := os.WriteFile(elsewhere, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := scanned(t, root, []diskmodel.Mount{{Path: filepath.Join(root, "keep", "mounted")}})

	err := scanner.Remove(filepath.Join(root, "keep"))
	if err == nil {
		t.Fatal("deleting a directory with another filesystem under it was allowed")
	}

	if !strings.Contains(err.Error(), "another filesystem") {
		t.Errorf("the refusal does not say why: %v", err)
	}

	if _, err := os.Stat(elsewhere); err != nil {
		t.Errorf("the other filesystem's contents were deleted anyway: %v", err)
	}
}

func TestDeletingRefusesTheMountPointItself(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	mounted := filepath.Join(root, "mnt")
	if err := os.MkdirAll(mounted, 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := scanned(t, root, []diskmodel.Mount{{Path: mounted}})

	if err := scanner.Remove(mounted); err == nil {
		t.Error("deleting a mount point was allowed")
	}
}

func TestDeletingRefusesTheScanRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := scanned(t, root, nil)

	if err := scanner.Remove(root); err == nil {
		t.Error("deleting the scan's own root was allowed")
	}

	if _, err := os.Stat(root); err != nil {
		t.Errorf("the scan root was deleted: %v", err)
	}
}

func TestDeletingStillWorksOnAnOrdinaryDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	target := filepath.Join(root, "junk")
	if err := os.MkdirAll(filepath.Join(target, "deep"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(target, "deep", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := scanned(t, root, nil)

	if err := scanner.Remove(target); err != nil {
		t.Fatalf("an ordinary directory would not delete: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("the directory is still there: %v", err)
	}
}

func TestDeletingRefusesAnythingOutsideTheRoot(t *testing.T) {
	t.Parallel()

	var (
		root    = t.TempDir()
		outside = t.TempDir()
	)

	keep := filepath.Join(outside, "keep")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := scanned(t, root, nil)

	if err := scanner.Remove(keep); err == nil {
		t.Error("deleting outside the scan root was allowed")
	}

	if _, err := os.Stat(keep); err != nil {
		t.Errorf("a file outside the root was deleted: %v", err)
	}
}

func TestDeletingCannotEscapeThroughASymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep")
	if err := os.Mkdir(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	important := filepath.Join(keep, "important")
	if err := os.WriteFile(important, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	err := diskusage.RemoveInside(root, filepath.Join(root, "link", "keep"), func() ([]diskmodel.Mount, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("deleting through a symlink outside the root was allowed")
	}
	if _, err := os.Stat(important); err != nil {
		t.Errorf("the file outside the root was deleted: %v", err)
	}
}

func TestDeletingThroughASymlinkedRootStillSeesMounts(t *testing.T) {
	t.Parallel()

	realRoot := t.TempDir()
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(realRoot, "target")
	mounted := filepath.Join(target, "mounted")
	if err := os.MkdirAll(mounted, 0o755); err != nil {
		t.Fatal(err)
	}
	important := filepath.Join(mounted, "important")
	if err := os.WriteFile(important, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := diskusage.RemoveInside(link, filepath.Join(link, "target"), func() ([]diskmodel.Mount, error) {
		return []diskmodel.Mount{{Path: mounted}}, nil
	})
	if err == nil {
		t.Fatal("a symlinked root hid the filesystem mounted under the target")
	}
	if _, err := os.Stat(important); err != nil {
		t.Errorf("the mounted contents were deleted: %v", err)
	}
}

func TestDeletingReadsMountsAgainAndFailsClosed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target")
	mounted := filepath.Join(target, "mounted")
	if err := os.MkdirAll(mounted, 0o755); err != nil {
		t.Fatal(err)
	}

	var calls int
	source := func() ([]diskmodel.Mount, error) {
		calls++
		return []diskmodel.Mount{{Path: mounted}}, nil
	}

	scanner := diskusage.NewScanner(source)
	settled(t, scanner, func() { scanner.Start(t.Context(), root) })
	if err := scanner.Remove(target); err == nil {
		t.Fatal("a mount present at delete time was ignored")
	}
	if calls < 2 {
		t.Fatalf("mount source was read %d times, want once for scan and once for delete", calls)
	}

	keep := filepath.Join(target, "keep")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := diskusage.NewScanner(func() ([]diskmodel.Mount, error) {
		return nil, errors.New("mount table unavailable")
	})
	failure := settled(t, broken, func() { broken.Start(t.Context(), root) })
	if !strings.Contains(failure.Err, "mount table unavailable") {
		t.Fatalf("scan did not fail closed: %+v", failure)
	}
	if err := broken.Remove(target); err == nil || !strings.Contains(err.Error(), "mount table unavailable") {
		t.Fatalf("delete did not fail closed: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("target changed after boundary lookup failed: %v", err)
	}
}
