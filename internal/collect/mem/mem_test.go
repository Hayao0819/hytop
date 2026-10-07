//go:build linux

package mem

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestCollectorConvertsKernelKiBAndDerivesUsage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	info := "MemTotal: 1000 kB\nMemAvailable: 400 kB\nCached: 200 kB\nSwapTotal: 500 kB\nSwapFree: 300 kB\nZswap: 25 kB\nZswapped: 75 kB\n"
	if err := os.WriteFile(filepath.Join(root, "meminfo"), []byte(info), 0o644); err != nil {
		t.Fatal(err)
	}

	sysRoot := t.TempDir()
	mmStat := filepath.Join(sysRoot, "block", "zram0", "mm_stat")
	if err := os.MkdirAll(filepath.Dir(mmStat), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mmStat, []byte("3000 1000 1200 0 0 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	collector, err := New(root, sysRoot)
	if err != nil {
		t.Fatal(err)
	}
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("availability = %+v", availability)
	}

	samples, err := collector.Collect(t.Context(), time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
	}

	for key, want := range map[series.Key]float64{
		"mem.total": 1000 * 1024, "mem.used": 600 * 1024,
		"mem.available": 400 * 1024, "mem.cached": 200 * 1024,
		"mem.usage": 60, "swap.total": 500 * 1024, "swap.used": 200 * 1024,
		"mem.zswap.compressed": 25 * 1024, "mem.zswap.stored": 75 * 1024,
		"mem.zram.original": 3000, "mem.zram.compressed": 1000,
		"mem.zram.used": 1200, "mem.zram.ratio": 3,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; samples=%v", key, got[key], want, got)
		}
	}
}
