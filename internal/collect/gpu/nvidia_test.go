//go:build linux

package gpu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNVIDIAMetricsJoinToTheDRMCardByPCIBus(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	samples, err := parseNVIDIA(
		[]byte("00000000:08:00.0, 12, 321, 12288, 42, 17.69, 210\n"),
		map[string]int{"0:08:00.0": 1},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]float64{}
	for _, sample := range samples {
		got[string(sample.Key)] = sample.Value
	}

	for key, want := range map[string]float64{
		"gpu.1.util":      12,
		"gpu.1.mem.used":  321 * 1024 * 1024,
		"gpu.1.mem.total": 12288 * 1024 * 1024,
		"gpu.1.temp":      42,
		"gpu.1.power":     17.69,
		"gpu.1.clock":     210e6,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; all samples: %v", key, got[key], want, got)
		}
	}
}

func TestCollectNVIDIAExecutesOncePerInterval(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := filepath.Join(dir, "nvidia-smi")
	args := filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + args +
		"\nprintf '00000000:08:00.0, 12, 321, 12288, 42, 17.69, 210\\n'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	collector := &Collector{
		cards: []card{{index: 2, busID: "0:08:00.0", nvidia: true}},
		smi:   script,
	}
	now := time.Unix(100, 0)
	samples, err := collector.collectNVIDIA(context.Background(), now)
	if err != nil || len(samples) != 6 {
		t.Fatalf("first collection = %d samples, %v", len(samples), err)
	}
	called, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(called), "--id=0:08:00.0") {
		t.Fatalf("nvidia-smi was not restricted to the awake card: %q", called)
	}
	samples, err = collector.collectNVIDIA(context.Background(), now.Add(time.Second))
	if err != nil || len(samples) != 0 {
		t.Fatalf("throttled collection = %d samples, %v", len(samples), err)
	}
}

func TestCollectNVIDIAAvailabilityErrors(t *testing.T) {
	t.Parallel()

	collector := &Collector{}
	if samples, err := collector.collectNVIDIA(context.Background(), time.Now()); err != nil || samples != nil {
		t.Fatalf("no NVIDIA card = %v, %v", samples, err)
	}
	collector.cards = []card{{nvidia: true}}
	if _, err := collector.collectNVIDIA(context.Background(), time.Now()); err == nil || !strings.Contains(err.Error(), "nvidia-smi") {
		t.Fatalf("missing nvidia-smi error = %v", err)
	}
}

func TestCollectNVIDIADoesNotWakeASleepingCard(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	device := filepath.Join(sysRoot, "devices", "0000:08:00.0")
	card := filepath.Join(sysRoot, "class", "drm", "card0")
	if err := os.MkdirAll(device, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(card, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(device, filepath.Join(card, "device")); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"power_state": "D3cold\n", "vendor": "0x10de\n"} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	collector := New(sysRoot, filepath.Join(root, "run"))
	collector.smi = filepath.Join(root, "does-not-exist")
	if samples, err := collector.collectNVIDIA(context.Background(), time.Now()); err != nil || samples != nil {
		t.Fatalf("sleeping NVIDIA card = %v, %v", samples, err)
	}
	if samples, err := collector.Collect(context.Background(), time.Now()); err != nil || samples != nil {
		t.Fatalf("sleeping GPU collector = %v, %v", samples, err)
	}
}

func TestNormalizeBusIDMatchesNVMLAndSysfsForms(t *testing.T) {
	t.Parallel()

	if a, b := normalizeBusID("00000000:08:00.0"), normalizeBusID("0000:08:00.0"); a != b {
		t.Fatalf("NVML bus %q != sysfs bus %q", a, b)
	}
}
