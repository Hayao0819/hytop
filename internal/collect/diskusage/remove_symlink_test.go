package diskusage_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

func TestDeletingASymlinkRemovesOnlyTheLink(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		directory bool
		outside   bool
		dangling  bool
		mounted   bool
	}{
		{name: "file"},
		{name: "directory", directory: true},
		{name: "outside root", outside: true},
		{name: "dangling", dangling: true},
		{name: "mounted target", directory: true, mounted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			targetRoot := root
			if tc.outside {
				targetRoot = t.TempDir()
			}
			target := filepath.Join(targetRoot, "keep")
			important := target
			if tc.directory {
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
				important = filepath.Join(target, "important.txt")
			}
			if !tc.dangling {
				if err := os.WriteFile(important, []byte("keep me"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			destination := target
			if !tc.outside {
				destination = filepath.Base(target)
			}
			link := filepath.Join(root, "shortcut")
			if err := os.Symlink(destination, link); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("creating a symlink requires privileges: %v", err)
				}
				t.Fatal(err)
			}
			mounts := func() ([]diskmodel.Mount, error) {
				if tc.mounted {
					return []diskmodel.Mount{{Path: target}}, nil
				}
				return nil, nil
			}
			if err := diskusage.RemoveInside(root, link, mounts); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(link); !os.IsNotExist(err) {
				t.Fatalf("selected symlink remains: %v", err)
			}
			if !tc.dangling {
				if contents, err := os.ReadFile(important); err != nil || string(contents) != "keep me" {
					t.Fatalf("link target changed: contents=%q err=%v", contents, err)
				}
			}
		})
	}
}
