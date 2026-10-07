//go:build linux

package hwmon

import (
	"os"
	"path/filepath"
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

func TestCollectorNamesAndScalesSensorChannels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "class/hwmon/hwmon0")
	write(t, filepath.Join(dir, "name"), "nct 6775\n")
	write(t, filepath.Join(dir, "temp1_input"), "42500\n")
	write(t, filepath.Join(dir, "temp1_label"), "CPU Package\n")
	write(t, filepath.Join(dir, "temp2_input"), "51000\n")
	write(t, filepath.Join(dir, "fan2_input"), "1350\n")
	write(t, filepath.Join(dir, "power1_average"), "22500000\n")
	write(t, filepath.Join(dir, "power1_input"), "25000000\n")
	write(t, filepath.Join(dir, "power2_input"), "3000000\n")

	collector := New(root)
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
		"thermal.nct_6775_CPU_Package.temp": 42.5,
		"thermal.nct_6775_temp2.temp":       51,
		"fan.nct_6775_fan2.rpm":             1350,
		"power.nct_6775_power1.watts":       22.5,
		"power.nct_6775_power2.watts":       3,
		"thermal.max.temp":                  51,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; samples=%v", key, got[key], want, got)
		}
	}
}
