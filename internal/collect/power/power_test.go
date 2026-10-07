//go:build linux

package power

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

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
	write(t, battery, "energy_now", "30000000\n")
	write(t, battery, "voltage_now", "12000000\n")
	write(t, battery, "cycle_count", "123\n")
	write(t, battery, "temp", "295\n")
	write(t, battery, "time_to_empty_now", "7200\n")
	write(t, battery, "charge_control_start_threshold", "40\n")
	write(t, battery, "charge_control_end_threshold", "80\n")
	write(t, battery, "manufacturer", "Acme\n")
	write(t, battery, "model_name", "Longlife\n")
	write(t, battery, "serial_number", "1234\n")
	write(t, battery, "technology", "Li-ion\n")
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
	want := map[series.Key]float64{
		"battery.0.capacity": 75, "battery.0.power": 12,
		"battery.0.energy": 30, "battery.0.energy_full": 45,
		"battery.0.energy_design": 50, "battery.0.health": 90,
		"battery.0.voltage": 12, "battery.0.cycles": 123,
		"battery.0.temp": 29.5, "battery.0.time_to_empty": 7200,
		"battery.0.charge_start": 40, "battery.0.charge_end": 80,
	}
	for _, sample := range samples {
		if expected, ok := want[sample.Key]; ok {
			if sample.Value != expected {
				t.Errorf("%s = %v, want %v", sample.Key, sample.Value, expected)
			}
			delete(want, sample.Key)
		}
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
	if facts["battery.name.0"] != "Longlife" || facts["battery.vendor.0"] != "Acme" {
		t.Fatalf("battery facts = %v", facts)
	}
	if got := collector.Batteries(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("Batteries() = %v", got)
	}
}

func TestHistorySamplesAreSortedAndBounded(t *testing.T) {
	t.Parallel()

	now := time.Unix(2_000_000, 0)
	history := []historyItem{
		{Time: uint32(now.Add(-time.Hour).Unix()), Value: 70},
		{Time: uint32(now.Add(-2 * time.Hour).Unix()), Value: 60},
		{Time: uint32(now.Add(-8 * 24 * time.Hour).Unix()), Value: 50},
		{Time: uint32(now.Add(time.Hour).Unix()), Value: 80},
	}

	samples := historySamples(2, history, now)
	if len(samples) != 2 || samples[0].Value != 60 || samples[1].Value != 70 {
		t.Fatalf("historySamples() = %v", samples)
	}
	if samples[0].Key != "battery.2.capacity" {
		t.Fatalf("history key = %q", samples[0].Key)
	}
}

func TestUPowerHistoryItemMatchesTheDBusTuple(t *testing.T) {
	t.Parallel()

	if got := dbus.SignatureOf(historyItem{}).String(); got != "(udu)" {
		t.Fatalf("historyItem signature = %q, want (udu)", got)
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
