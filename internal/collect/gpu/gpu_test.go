//go:build linux

package gpu

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestCardsAndNamesIncludeGPUsWithoutSysfsCounters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	runRoot := filepath.Join(root, "run")
	drmRoot := filepath.Join(sysRoot, "class", "drm")

	for index, slot := range []string{"0000:03:00.0", "0000:08:00.0"} {
		device := filepath.Join(sysRoot, "devices", "pci0000:00", slot)
		if err := os.MkdirAll(device, 0o755); err != nil {
			t.Fatal(err)
		}
		card := filepath.Join(drmRoot, "card"+strconv.Itoa(index))
		if err := os.MkdirAll(card, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(device, filepath.Join(card, "device")); err != nil {
			t.Fatal(err)
		}
	}

	database := filepath.Join(runRoot, "udev", "data")
	if err := os.MkdirAll(database, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(database, "+pci:0000:03:00.0"), []byte("E:ID_MODEL_FROM_DATABASE=Radeon RX Vega 64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(database, "+pci:0000:08:00.0"), []byte("E:ID_MODEL_FROM_DATABASE=GeForce RTX 3060\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	collector := New(sysRoot, runRoot)
	if got := collector.Cards(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("Cards() = %v", got)
	}

	facts, err := collector.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if facts[series.FactGPUName+".0"] != "Radeon RX Vega 64" || facts[series.FactGPUName+".1"] != "GeForce RTX 3060" {
		t.Fatalf("GPU facts = %#v", facts)
	}
}

func TestCheckFindsACardThatAppearsAfterStartup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	collector := New(sysRoot, filepath.Join(root, "run"))
	if collector.Check().OK() {
		t.Fatal("an empty DRM directory was reported ready")
	}

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

	if availability := collector.Check(); !availability.OK() {
		t.Fatalf("new card remained unavailable: %+v", availability)
	}
	if got := collector.Cards(); !slices.Equal(got, []int{0}) {
		t.Fatalf("Cards() = %v, want [0]", got)
	}
}

func TestCardDiscoveryUsesItsOwnInterval(t *testing.T) {
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

	collector := New(sysRoot, filepath.Join(root, "run"))
	discovered := collector.lastDiscovery
	if err := os.RemoveAll(card); err != nil {
		t.Fatal(err)
	}

	_, _ = collector.Collect(t.Context(), discovered.Add(cardDiscoveryInterval-time.Millisecond))
	if got := collector.Cards(); !slices.Equal(got, []int{0}) {
		t.Fatalf("Cards() before discovery interval = %v, want [0]", got)
	}

	_, _ = collector.Collect(t.Context(), discovered.Add(cardDiscoveryInterval))
	if got := collector.Cards(); len(got) != 0 {
		t.Fatalf("Cards() after discovery interval = %v, want none", got)
	}
	facts, err := collector.Facts(t.Context())
	if err != nil || len(facts) != 0 {
		t.Fatalf("Facts() after removal = %v, %v", facts, err)
	}
}

func TestSysfsCollectsGTTAndHwmonDetails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sysRoot := filepath.Join(root, "sys")
	device := filepath.Join(sysRoot, "devices", "0000:03:00.0")
	card := filepath.Join(sysRoot, "class", "drm", "card0")
	if err := os.MkdirAll(filepath.Join(device, "hwmon", "hwmon0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(card, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(device, filepath.Join(card, "device")); err != nil {
		t.Fatal(err)
	}

	values := map[string]string{
		"mem_info_gtt_used": "1024\n", "mem_info_gtt_total": "4096\n",
		"current_link_speed": "8.0 GT/s PCIe\n", "current_link_width": "16\n",
		"max_link_speed": "16.0 GT/s PCIe\n", "max_link_width": "16\n",
		"hwmon/hwmon0/power1_input": "3000000\n",
		"hwmon/hwmon0/fan1_input":   "1200\n",
		"hwmon/hwmon0/freq2_label":  "mclk\n",
		"hwmon/hwmon0/freq2_input":  "167000000\n",
	}
	for name, value := range values {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	collector := New(sysRoot, filepath.Join(root, "run"))
	samples, err := collector.Collect(t.Context(), time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
	}
	for key, want := range map[series.Key]float64{
		"gpu.0.mem.gtt.used": 1024, "gpu.0.mem.gtt.total": 4096,
		"gpu.0.power": 3, "gpu.0.fan.rpm": 1200, "gpu.0.mem.clock": 167000000,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v", key, got[key], want)
		}
	}

	facts, err := collector.Facts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if facts["gpu.0.pcie.current"] != "8.0 GT/s PCIe ×16" ||
		facts["gpu.0.pcie.max"] != "16.0 GT/s PCIe ×16" {
		t.Fatalf("PCIe facts = %v", facts)
	}
}
