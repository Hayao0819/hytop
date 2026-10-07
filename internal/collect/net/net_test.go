//go:build linux

package net

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"
)

func TestInterfacesAndSpeedComeFromTypedSysfsData(t *testing.T) {
	procRoot := t.TempDir()
	sysRoot := t.TempDir()

	mustWrite(t, filepath.Join(procRoot, "net", "dev"), "Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n  eth0: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n  tap0: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n")
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "carrier"), "1\n")
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "speed"), "2500\n")
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "tap0", "carrier"), "0\n")

	collector, err := New(procRoot, sysRoot)
	if err != nil {
		t.Fatal(err)
	}

	if got := collector.Interfaces(); len(got) != 1 || got[0] != "eth0" {
		t.Fatalf("Interfaces() = %v, want [eth0]", got)
	}

	_, _ = collector.Collect(t.Context(), time.Unix(1, 0))
	samples, err := collector.Collect(t.Context(), time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}

	for _, sample := range samples {
		if sample.Key == "net.eth0.speed" && sample.Value == 2.5e9 {
			return
		}
	}

	t.Fatalf("samples = %v, want net.eth0.speed=2.5e9", samples)
}

func TestCounterResetStartsANewRateBaseline(t *testing.T) {
	t.Parallel()

	procRoot := t.TempDir()
	sysRoot := t.TempDir()
	device := func(rx, tx int) string {
		return "Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n" +
			fmt.Sprintf(" eth0: %d 0 0 0 0 0 0 0 %d 0 0 0 0 0 0 0\n", rx, tx)
	}
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "carrier"), "1\n")
	mustWrite(t, filepath.Join(procRoot, "net", "dev"), device(1000, 500))

	collector, err := New(procRoot, sysRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = collector.Collect(t.Context(), time.Unix(1, 0))

	mustWrite(t, filepath.Join(procRoot, "net", "dev"), device(10, 5))
	samples, err := collector.Collect(t.Context(), time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 0 {
		t.Fatalf("counter reset produced rates: %v", samples)
	}

	mustWrite(t, filepath.Join(procRoot, "net", "dev"), device(30, 15))
	samples, err = collector.Collect(t.Context(), time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range samples {
		if sample.Key == "net.eth0.rx" && sample.Value == 20 {
			return
		}
	}
	t.Fatalf("new baseline did not produce the next rate: %v", samples)
}

func TestLinkStateAvoidsReparsingSysfsOnEverySample(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sysRoot := t.TempDir()
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "carrier"), "1\n")
	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "speed"), "2500\n")

	collector, err := New(root, sysRoot)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Unix(100, 0)
	if got, ok := speed(collector.linkState(start)["eth0"]); !ok || got != 2.5e9 {
		t.Fatalf("initial speed = %v, %v", got, ok)
	}

	mustWrite(t, filepath.Join(sysRoot, "class", "net", "eth0", "speed"), "1000\n")
	if got, _ := speed(collector.linkState(start.Add(time.Second))["eth0"]); got != 2.5e9 {
		t.Fatalf("cached speed = %v, want 2.5e9", got)
	}
	if got, _ := speed(collector.linkState(start.Add(linkRefresh))["eth0"]); got != 1e9 {
		t.Fatalf("refreshed speed = %v, want 1e9", got)
	}
}

func TestInterfacesPreferCurrentTrafficOverLifetimeTotals(t *testing.T) {
	t.Parallel()

	up := int64(1)
	devices := procfs.NetDev{
		"historical": {RxBytes: 1_000_000},
		"active":     {RxBytes: 100},
	}
	links := sysfs.NetClass{
		"historical": {Carrier: &up},
		"active":     {Carrier: &up},
	}

	got := interfaces(devices, links, map[string]uint64{"active": 50})
	if len(got) != 2 || got[0] != "active" {
		t.Fatalf("interfaces() = %v", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
