//go:build linux

package diskusage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

// tree exceeds the scanner's cache threshold in each directory.
func tree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, dir := range []string{"keep", "keep/deep", "change"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}

		for i := range 12 {
			write(t, filepath.Join(root, dir, fmt.Sprintf("f%02d", i)), 16<<10)
		}
	}

	return root
}

func write(t *testing.T, path string, size int) {
	t.Helper()

	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func settled(t *testing.T, s *diskusage.Scanner, run func()) diskmodel.Scan {
	t.Helper()

	run()

	deadline := time.After(10 * time.Second)

	for {
		if scan := s.Scan(); scan.Done {
			return scan
		}

		select {
		case <-deadline:
			t.Fatal("the scan never finished")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func entry(t *testing.T, scan diskmodel.Scan, name string) diskmodel.Entry {
	t.Helper()

	for _, got := range scan.Entries {
		if got.Name == name {
			return got
		}
	}

	t.Fatalf("no entry called %q in %v", name, scan.Entries)

	return diskmodel.Entry{}
}

func TestASecondLookReusesWhatHasNotChanged(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if first.Reused != 0 {
		t.Errorf("the first look reused %d directories; there was nothing to reuse", first.Reused)
	}

	if got := entry(t, first, "keep").Size; got == 0 {
		t.Fatalf("keep came to nothing:\n%v", first.Entries)
	}

	second := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if second.Reused < 2 {
		t.Errorf("the second look reused %d directories, want at least 2", second.Reused)
	}

	if first.Total != second.Total {
		t.Errorf("the total changed from %d to %d without the tree changing", first.Total, second.Total)
	}
}

func TestADirectoryThatChangedIsCountedAgain(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	write(t, filepath.Join(root, "change", "extra"), 128<<10)

	second := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if second.Total <= first.Total {
		t.Errorf("the total is %d after adding 128 KiB to %d", second.Total, first.Total)
	}

	if got := entry(t, second, "change").Size; got <= entry(t, first, "change").Size {
		t.Error("the directory that gained a file was answered from the cache")
	}

	if entry(t, second, "keep").Size != entry(t, first, "keep").Size {
		t.Error("an untouched directory came to something different")
	}
}

func TestAFileThatGrewInPlaceNeedsAFullMeasure(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	write(t, filepath.Join(root, "keep", "deep", "f00"), 512<<10)

	quick := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if quick.Total != first.Total {
		t.Logf("the quick look noticed the growth (%d → %d), which is more than it promises",
			first.Total, quick.Total)
	}

	full := settled(t, scanner, func() { scanner.Restart(ctx, root) })

	if full.Total <= first.Total {
		t.Errorf("a full measure still says %d, was %d", full.Total, first.Total)
	}

	if full.Reused != 0 {
		t.Errorf("a full measure reused %d directories; it should reuse none", full.Reused)
	}
}

func TestDeletingIsNoticedWithoutInvalidatingAnything(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if err := scanner.Remove(filepath.Join(root, "keep", "deep")); err != nil {
		t.Fatal(err)
	}

	second := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if second.Total >= first.Total {
		t.Errorf("the total is %d after deleting from %d; the old answer was kept",
			second.Total, first.Total)
	}
}

func TestAWalkStopsAtAnotherFilesystem(t *testing.T) {
	t.Parallel()

	root := tree(t)

	scanner := diskusage.NewScanner(func() ([]diskmodel.Mount, error) {
		return []diskmodel.Mount{{Path: filepath.Join(root, "keep")}}, nil
	})

	scan := settled(t, scanner, func() { scanner.Start(t.Context(), root) })

	keep := entry(t, scan, "keep")
	if !keep.Mount {
		t.Errorf("keep is a mount point and was not marked as one: %+v", keep)
	}

	if keep.Size != 0 {
		t.Errorf("keep came to %d; another filesystem's size belongs to its own row", keep.Size)
	}
}

func TestRemoveRefusesAnythingOutsideTheScan(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)

	settled(t, scanner, func() { scanner.Start(t.Context(), filepath.Join(root, "keep")) })

	outside := filepath.Join(root, "change")

	if err := scanner.Remove(outside); err == nil {
		t.Fatal("a path outside the scan should be refused")
	}

	if _, err := os.Stat(outside); err != nil {
		t.Errorf("it was refused and deleted anyway: %v", err)
	}
}

func TestScanningSomethingUnreadableDoesNotCacheAWrongTotal(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root can read a directory with no permissions")
	}

	root := t.TempDir()
	shut := filepath.Join(root, "shut")

	if err := os.MkdirAll(filepath.Join(shut, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(shut, "inside", "a"), 64<<10)

	if err := os.Chmod(filepath.Join(shut, "inside"), 0o000); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(filepath.Join(shut, "inside"), 0o755) })

	scanner := diskusage.NewScanner(nil)
	ctx := context.Background()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })
	if !strings.Contains(strings.ToLower(first.Err), "permission denied") {
		t.Fatalf("an incomplete scan reported no permission error: %+v", first)
	}

	if err := os.Chmod(filepath.Join(shut, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}

	second := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if entry(t, second, "shut").Size == 0 {
		t.Errorf("the unreadable subtree was cached as empty:\n%v", second.Entries)
	}
}

func TestAFileAddedDeeperIsNoticed(t *testing.T) {
	t.Parallel()

	root := tree(t)
	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	keep := filepath.Join(root, "keep")
	was, err := os.Stat(keep)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(keep, "deep", "new"), 256<<10)

	now, err := os.Stat(keep)
	if err != nil {
		t.Fatal(err)
	}

	if !was.ModTime().Equal(now.ModTime()) {
		t.Fatal("keep's mtime moved, so this test is not testing what it says")
	}

	second := settled(t, scanner, func() { scanner.Start(ctx, root) })

	if second.Total <= first.Total {
		t.Fatalf("total is %d after adding 256 KiB deeper, was %d — the subtree was reused",
			second.Total, first.Total)
	}
}

func TestTheSameTreeComesToTheSameNumberEveryTime(t *testing.T) {
	t.Parallel()

	root := tree(t)

	target := filepath.Join(root, "keep", "f00")

	for i := range 4 {
		if err := os.Link(target, filepath.Join(root, "change", fmt.Sprintf("link%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	scanner := diskusage.NewScanner(nil)
	ctx := t.Context()

	first := settled(t, scanner, func() { scanner.Start(ctx, root) })

	for i := range 5 {
		again := settled(t, scanner, func() { scanner.Start(ctx, root) })

		if again.Total != first.Total || again.Items != first.Items {
			t.Fatalf("look %d came to %d bytes / %d items, the first came to %d / %d",
				i+2, again.Total, again.Items, first.Total, first.Items)
		}
	}

	full := settled(t, scanner, func() { scanner.Restart(ctx, root) })

	if full.Total != first.Total || full.Items != first.Items {
		t.Errorf("measuring everything came to %d / %d, reusing came to %d / %d",
			full.Total, full.Items, first.Total, first.Items)
	}
}

func TestADeepTreeIsCountedWithoutDeadlocking(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	at := root

	for range 64 {
		at = filepath.Join(at, "down")
		if err := os.Mkdir(at, 0o755); err != nil {
			t.Fatal(err)
		}

		write(t, filepath.Join(at, "f"), 8<<10)
	}

	scanner := diskusage.NewScanner(nil)

	done := make(chan diskmodel.Scan, 1)

	go func() { done <- settled(t, scanner, func() { scanner.Start(t.Context(), root) }) }()

	select {
	case scan := <-done:
		if scan.Items != 64 {
			t.Errorf("counted %d files down a 64-deep tree", scan.Items)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the walk never finished; a worker is waiting on a slot its ancestor holds")
	}
}
