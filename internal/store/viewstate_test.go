package store_test

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/store"
)

func TestViewStateTransitions(t *testing.T) {
	t.Parallel()

	state := store.NewViewState()
	if by, descending := state.Sort(); by != store.SortCPU || !descending {
		t.Fatalf("initial sort = %q, %v", by, descending)
	}

	state.SetSelected(-3)
	state.SetOffset(-4)
	if state.Selected() != 0 || state.Offset() != 0 {
		t.Fatalf("negative position = %d, %d", state.Selected(), state.Offset())
	}

	state.ToggleTree()
	state.ToggleKernel()
	state.ToggleLogs()
	process42 := procmodel.Identity{PID: 42, Started: 1}
	process99 := procmodel.Identity{PID: 99, Started: 1}
	state.ToggleCollapsed(process42)
	if !state.Tree() || !state.Kernel() || !state.Logs() || !state.Collapsed(process42) {
		t.Fatal("toggles did not enable state")
	}
	state.ToggleCollapsed(process42)
	if state.Collapsed(process42) {
		t.Fatal("collapsed PID was not removed")
	}
	state.ToggleCollapsed(process42)
	state.ToggleCollapsed(process99)
	state.PruneCollapsed([]procmodel.Identity{process42})
	if !state.Collapsed(process42) || state.Collapsed(process99) {
		t.Fatal("collapsed state was not pruned to live PIDs")
	}
	state.PruneCollapsed([]procmodel.Identity{{PID: 42, Started: 2}})
	if state.Collapsed(process42) {
		t.Fatal("collapsed state followed a reused PID")
	}

	state.SelectProcess(1, procmodel.Identity{PID: 42, Started: 7})
	if got := state.ReconcileProcesses([]procmodel.Identity{{PID: 9}, {PID: 8}, {PID: 42, Started: 7}}); got != 2 {
		t.Fatalf("reconciled selection = %d, want 2", got)
	}
	if got := state.ReconcileProcesses([]procmodel.Identity{{PID: 42, Started: 8}, {PID: 8}}); got != 1 {
		t.Fatalf("reused PID selection = %d, want nearest row 1", got)
	}

	state.SetSort(store.SortCPU)
	if _, descending := state.Sort(); descending {
		t.Fatal("same sort did not flip direction")
	}
	state.SetSort(store.SortName)
	if by, descending := state.Sort(); by != store.SortName || !descending {
		t.Fatalf("new sort = %q, %v", by, descending)
	}
	state.SetSortOrder(store.SortPID, false)
	if by, descending := state.Sort(); by != store.SortPID || descending {
		t.Fatalf("explicit sort = %q, %v", by, descending)
	}

	expr, err := filter.Compile(`pid == 7`)
	if err != nil {
		t.Fatal(err)
	}
	state.SetFilter("old", expr)
	state.SetSelected(9)
	state.SetOffset(5)
	state.PushFilter("new", expr)
	if source, got := state.Filter(); source != "new" || got == nil || state.Selected() != 0 || state.Offset() != 0 {
		t.Fatalf("pushed filter = %q, %v", source, got)
	}
	if previous, ok := state.PopFilter(); !ok || previous != "old" {
		t.Fatalf("PopFilter = %q, %v", previous, ok)
	}
	if _, ok := state.PopFilter(); ok {
		t.Fatal("empty filter stack popped")
	}

	state.SetServiceSearch("failed")
	if state.ServiceSearch() != "failed" {
		t.Fatal("service search was not stored")
	}
	state.SetTree(false)
	state.SetKernel(false)
	state.SetLogs(false)
	if state.Tree() || state.Kernel() || state.Logs() {
		t.Fatal("explicit setters did not disable state")
	}
}
