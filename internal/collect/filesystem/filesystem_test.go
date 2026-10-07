//go:build linux

package filesystem_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect/filesystem"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

// procRoot creates a mount table whose entries survive the collector's statfs check.
func procRoot(t *testing.T, table string) string {
	t.Helper()

	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "self"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "self", "mountinfo"), []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func collect(t *testing.T, table string) map[series.Key]float64 {
	t.Helper()

	samples, err := filesystem.New(procRoot(t, table)).Collect(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
	}

	return got
}

func line(device, path, fstype string) string {
	return "1 0 0:1 / " + path + " rw,relatime - " + fstype + " " + device + " rw\n"
}

func TestMemoryBackedFilesystemsAreNotCapacity(t *testing.T) {
	t.Parallel()

	var (
		here  = t.TempDir()
		alone = collect(t, line("/dev/sda1", here, "ext4"))
		both  = collect(t, line("/dev/sda1", here, "ext4")+line("tmpfs", t.TempDir(), "tmpfs"))
	)

	if alone["fs.total.size"] == 0 {
		t.Fatal("the one real filesystem was not counted at all")
	}

	if both["fs.total.size"] != alone["fs.total.size"] {
		t.Errorf("tmpfs was added to the storage total: %v with it, %v without",
			both["fs.total.size"], alone["fs.total.size"])
	}
}

func TestAMemoryBackedFilesystemStillGetsItsOwnRow(t *testing.T) {
	t.Parallel()

	shm := t.TempDir()

	got := collect(t, line("tmpfs", shm, "tmpfs"))

	if _, ok := got[series.Key("fs."+filesystem.Escape(shm)+".usage")]; !ok {
		t.Errorf("tmpfs was dropped from the list as well as from the total:\n%v", got)
	}
}

func TestBoundariesKeepFilesystemsHiddenFromTheCapacityTable(t *testing.T) {
	t.Parallel()

	root := procRoot(t, line("proc", "/proc", "proc")+line("devtmpfs", "/dev", "devtmpfs")+
		line("/dev/loop0", "/snap/tool", "squashfs"))

	mounts, err := filesystem.Boundaries(root)
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]bool, len(mounts))
	for _, mount := range mounts {
		got[mount.Path] = true
	}

	for _, want := range []string{"/proc", "/dev", "/snap/tool"} {
		if !got[want] {
			t.Errorf("boundary list dropped %s: %v", want, mounts)
		}
	}
}

func TestMountPathsDoNotShareMetricKeys(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]string{{"/a-b", "/a/b"}, {"/a.b", "/a_b"}, {"/", "/root"}} {
		if left, right := filesystem.Escape(pair[0]), filesystem.Escape(pair[1]); left == right {
			t.Errorf("Escape(%q) and Escape(%q) both returned %q", pair[0], pair[1], left)
		}
	}
}

func TestADeviceMountedTwiceIsCountedOnce(t *testing.T) {
	t.Parallel()

	var (
		once  = collect(t, line("/dev/sda1", t.TempDir(), "ext4"))
		twice = collect(t, line("/dev/sda1", t.TempDir(), "ext4")+line("/dev/sda1", t.TempDir(), "ext4"))
	)

	if twice["fs.total.size"] != once["fs.total.size"] {
		t.Errorf("the second mount of the same device was counted again: %v then %v",
			once["fs.total.size"], twice["fs.total.size"])
	}
}

func TestTheTotalIsTheSumOfDistinctDevices(t *testing.T) {
	t.Parallel()

	var (
		one = collect(t, line("/dev/sda1", t.TempDir(), "ext4"))
		two = collect(t, line("/dev/sda1", t.TempDir(), "ext4")+line("/dev/sdb1", t.TempDir(), "ext4"))
	)

	if two["fs.total.size"] != 2*one["fs.total.size"] {
		t.Errorf("two devices on the same disk should total twice one: %v then %v",
			one["fs.total.size"], two["fs.total.size"])
	}
}
