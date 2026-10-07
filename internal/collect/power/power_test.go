//go:build linux

package power

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestPowerSupplyClassDrivesSamplesAndFacts(t *testing.T) {
	root := t.TempDir()
	battery := filepath.Join(root, "class", "power_supply", "BAT0")
	mains := filepath.Join(root, "class", "power_supply", "AC")

	write(t, battery, "type", "Battery\n")
	write(t, battery, "capacity", "75\n")
	write(t, battery, "power_now", "12000000\n")
	write(t, battery, "energy_full", "45000000\n")
	write(t, battery, "energy_full_design", "50000000\n")
	write(t, battery, "status", "Discharging\n")
	write(t, mains, "type", "Mains\n")
	write(t, mains, "online", "0\n")

	collector := New(root)
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("Check() = %+v", availability)
	}

	samples, err := collector.Collect(t.Context(), time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := map[series.Key]float64{"battery.0.capacity": 75, "battery.0.power": 12}
	for _, sample := range samples {
		delete(want, sample.Key)
	}
	if len(want) != 0 {
		t.Fatalf("missing samples %v from %v", want, samples)
	}

	facts, err := collector.Facts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if facts[series.FactBatteryHealth] != "90 %" || facts[series.FactACOnline] != "on battery" {
		t.Fatalf("Facts() = %v", facts)
	}
}

func TestMainsOnlySupplyIsAValidCollector(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mains := filepath.Join(root, "class", "power_supply", "AC")
	write(t, mains, "type", "Mains\n")
	write(t, mains, "online", "1\n")

	collector := New(root)
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("Check() = %+v", availability)
	}
	if samples, err := collector.Collect(t.Context(), time.Unix(1, 0)); err != nil || len(samples) != 0 {
		t.Fatalf("Collect() = %v, %v", samples, err)
	}
	facts, err := collector.Facts(t.Context())
	if err != nil || facts[series.FactACOnline] != "connected" {
		t.Fatalf("Facts() = %v, %v", facts, err)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
