//go:build linux

package cpu

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stat(user, system, idle int) string {
	return "cpu  " + strconv.Itoa(user) + " 0 " + strconv.Itoa(system) + " " + strconv.Itoa(idle) + " 0 0 0 0 0 0\n" +
		"cpu0 " + strconv.Itoa(user) + " 0 " + strconv.Itoa(system) + " " + strconv.Itoa(idle) + " 0 0 0 0 0 0\n" +
		"btime 100\nprocs_running 1\nprocs_blocked 2\n"
}

func TestCollectorReadsRatesClocksTemperatureAndFacts(t *testing.T) {
	t.Parallel()

	procRoot, sysRoot := t.TempDir(), t.TempDir()
	write(t, filepath.Join(procRoot, "stat"), stat(100, 50, 850))
	write(t, filepath.Join(procRoot, "cpuinfo"), "processor: 0\nvendor_id: GenuineIntel\nmodel name: Fixture CPU\nphysical id: 0\ncore id: 0\ncpu MHz: 2500\nflags: fpu vmx\n")
	write(t, filepath.Join(sysRoot, "devices/system/cpu/present"), "0\n")
	write(t, filepath.Join(sysRoot, "devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"), "2000000\n")
	write(t, filepath.Join(sysRoot, "devices/system/cpu/cpu0/cpufreq/base_frequency"), "3600000\n")
	write(t, filepath.Join(sysRoot, "class/hwmon/hwmon0/name"), "coretemp\n")
	write(t, filepath.Join(sysRoot, "class/hwmon/hwmon0/temp1_input"), "55000\n")

	collector, err := New(procRoot, sysRoot)
	if err != nil {
		t.Fatal(err)
	}
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("availability = %+v", availability)
	}

	_, _ = collector.Collect(t.Context(), time.Unix(1000, 0))
	write(t, filepath.Join(procRoot, "stat"), stat(130, 70, 900))
	samples, err := collector.Collect(t.Context(), time.Unix(1001, 0))
	if err != nil {
		t.Fatal(err)
	}

	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
	}
	for key, want := range map[series.Key]float64{
		"cpu.total.usage": 50, "cpu.core.0.usage": 50,
		"cpu.total.freq": 2e9, "cpu.core.0.freq": 2e9,
		"cpu.package.temp": 55, "proc.blocked": 2, "system.uptime": 901,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; samples=%v", key, got[key], want, got)
		}
	}

	facts, err := collector.Facts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if facts[series.FactCPUModel] != "Fixture CPU" || facts[series.FactCPUVirtualisation] != "VT-x" ||
		facts[series.FactCPUBaseSpeed] != "3.60 GHz" {
		t.Fatalf("facts = %v", facts)
	}
}
