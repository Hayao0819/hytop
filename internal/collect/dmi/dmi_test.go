//go:build linux

package dmi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestCollectorReadsMemoryModulesFromTheUdevDatabase(t *testing.T) {
	t.Parallel()

	runRoot, sysRoot := t.TempDir(), t.TempDir()
	database := filepath.Join(runRoot, "udev/data/+dmi:id")
	if err := os.MkdirAll(filepath.Dir(database), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "E:MEMORY_ARRAY_MAX_CAPACITY=68719476736\n" +
		"E:MEMORY_DEVICE_0_SIZE=17179869184\n" +
		"E:MEMORY_DEVICE_0_FORM_FACTOR=DIMM\n" +
		"E:MEMORY_DEVICE_0_TYPE=DDR4\n" +
		"E:MEMORY_DEVICE_0_CONFIGURED_SPEED_MTS=3200\n" +
		"E:MEMORY_DEVICE_0_PART_NUMBER=Fixture RAM  \n" +
		"E:MEMORY_DEVICE_1_SIZE=17179869184\n" +
		"E:MEMORY_DEVICE_1_FORM_FACTOR=DIMM\n" +
		"E:MEMORY_DEVICE_1_TYPE=DDR4\n" +
		"E:MEMORY_DEVICE_2_SIZE=0\n"
	if err := os.WriteFile(database, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	collector := New(runRoot, sysRoot)
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("availability = %+v", availability)
	}
	facts, err := collector.Facts(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	for key, want := range map[string]string{
		series.FactMemorySlots:       "3",
		series.FactMemorySlotsUsed:   "2",
		series.FactMemoryMaxCapacity: "64 GiB",
		series.FactMemoryForm:        "DIMM",
		series.FactMemoryType:        "DDR4",
		series.FactMemorySpeed:       "3200 MT/s",
		series.FactMemoryPart:        "Fixture RAM",
		series.FactMemoryModules:     "2 × 16 GiB",
	} {
		if facts[key] != want {
			t.Errorf("%s = %q, want %q; facts=%v", key, facts[key], want, facts)
		}
	}
}
