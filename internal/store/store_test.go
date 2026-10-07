package store_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/store"
)

func TestStoreSnapshotsFactsAndGeneration(t *testing.T) {
	t.Parallel()

	memory := store.New([]metric.Resolution{{Interval: time.Second, Retention: time.Minute}})
	if memory.Generation() != 0 || memory.Processes().Len() != 0 {
		t.Fatal("new store is not empty")
	}

	memory.WriteProcesses([]procmodel.Process{{PID: 7, Name: "worker"}})
	if memory.Generation() != 1 || memory.Processes().Len() != 1 {
		t.Fatal("process write did not replace the snapshot")
	}

	memory.WriteFacts(map[string]string{"cpu.model": "test"})
	if value, ok := memory.Fact("cpu.model"); !ok || value != "test" || memory.Generation() != 2 {
		t.Fatalf("fact = %q, %v generation=%d", value, ok, memory.Generation())
	}
	if _, ok := memory.Fact("missing"); ok {
		t.Fatal("missing fact exists")
	}
	memory.WriteFacts(map[string]string{"hostile": "model\x1b[2J"})
	if value, _ := memory.Fact("hostile"); value != "model?[2J" {
		t.Fatalf("external fact was stored as %q", value)
	}
	if window := memory.Window("missing", time.Minute); window != nil {
		t.Fatalf("missing window = %v", window)
	}
}

func TestReplaceFactsRemovesOnlyTheSourcesMissingValues(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "one", "gpu.name.1": "two"})
	memory.ReplaceFacts("power", map[string]string{"power.ac": "connected"})
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "one"})

	if _, ok := memory.Fact("gpu.name.1"); ok {
		t.Fatal("a fact omitted by its source remained in the store")
	}
	if value, ok := memory.Fact("power.ac"); !ok || value != "connected" {
		t.Fatalf("replacing GPU facts changed another source: %q, %v", value, ok)
	}
}

func TestUnchangedFactsDoNotAdvanceTheGeneration(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "one"})
	generation := memory.Generation()
	memory.ReplaceFacts("gpu", map[string]string{"gpu.name.0": "one"})

	if memory.Generation() != generation {
		t.Fatal("an unchanged fact replacement advanced the generation")
	}
}
