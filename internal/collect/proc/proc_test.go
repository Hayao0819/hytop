//go:build linux

package proc

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/procfs"
)

func TestCollectorUsesTheKernelPageSize(t *testing.T) {
	t.Parallel()

	collector, err := New("/proc", os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if collector.pageSize != uint64(os.Getpagesize()) {
		t.Fatalf("page size = %d, want %d", collector.pageSize, os.Getpagesize())
	}
}

func TestUIDSaturatesInvalidValues(t *testing.T) {
	t.Parallel()

	if got := uid(1000); got != 1000 {
		t.Fatalf("uid(1000) = %d", got)
	}
	if got := uid(math.MaxUint64); got != math.MaxInt64 {
		t.Fatalf("uid(max) = %d, want %d", got, int64(math.MaxInt64))
	}
}

func TestStatusUIDReadsTheRealUID(t *testing.T) {
	t.Parallel()

	contents := []byte("Name:\thytop\nState:\tR (running)\nUid:\t1000\t1001\t1002\t1003\n")
	got, ok := statusUID(contents)
	if !ok || got != 1000 {
		t.Fatalf("statusUID() = %d, %v, want 1000, true", got, ok)
	}

	for _, invalid := range [][]byte{
		[]byte("Name:\thytop\n"),
		[]byte("Uid:\n"),
		[]byte("Uid:\tinvalid\n"),
	} {
		if got, ok := statusUID(invalid); ok {
			t.Errorf("statusUID(%q) = %d, true", invalid, got)
		}
	}
}

func TestProcFileReaders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	status := filepath.Join(dir, "status")
	if err := os.WriteFile(status, []byte("Name:\thytop\nState:\tR\nUid:\t1000\t1000\t1000\t1000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := readStatusUID(status, make([]byte, 128)); !ok || got != 1000 {
		t.Fatalf("readStatusUID() = %d, %v", got, ok)
	}

	want := strings.Repeat("0123456789", 1000)
	path := filepath.Join(dir, "long")
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readSmallFile(path, make([]byte, 128))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("readSmallFile() read %d bytes, want %d", len(got), len(want))
	}
}

func TestProcessScanHonorsCancellation(t *testing.T) {
	t.Parallel()

	collector, err := New("/proc", os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := collector.Processes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Processes() error = %v, want context cancellation", err)
	}
}

func TestOwnCPUUsesDeltasAndRejectsBackwardsTime(t *testing.T) {
	t.Parallel()

	collector := &Collector{clockTicks: 100}
	start := time.Unix(100, 0)
	if got := collector.ownCPU(procfs.ProcStat{UTime: 100}, start); got != 0 {
		t.Fatalf("first CPU sample = %v", got)
	}
	if got := collector.ownCPU(procfs.ProcStat{UTime: 150, STime: 50}, start.Add(time.Second)); got != 100 {
		t.Fatalf("CPU delta = %v, want 100", got)
	}
	if got := collector.ownCPU(procfs.ProcStat{UTime: 200, STime: 50}, start); got != 0 {
		t.Fatalf("backwards CPU sample = %v", got)
	}
}

func TestUnitOfFindsSystemdServiceAndScope(t *testing.T) {
	t.Parallel()

	for source, want := range map[string]string{
		"/system.slice/sshd.service":                  "sshd.service",
		"/user.slice/user-1000.slice/session-3.scope": "session-3.scope",
		"/docker/abcdef":                              "",
	} {
		if got := unitOf(source); got != want {
			t.Errorf("unitOf(%q) = %q, want %q", source, got, want)
		}
	}
}
