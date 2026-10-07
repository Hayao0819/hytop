//go:build linux

package proc

import (
	"context"
	"errors"
	"math"
	"os"
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
