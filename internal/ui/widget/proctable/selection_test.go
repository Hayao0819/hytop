package proctable

import (
	"testing"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func TestSelectionFollowsProcessIdentityAcrossSortingAndRefresh(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	view := store.NewViewState()
	memory.WriteProcesses([]procmodel.Process{
		{PID: 10, Started: 1, Name: "first", CPU: 90},
		{PID: 20, Started: 2, Name: "second", CPU: 10},
	})
	table := New(memory, view, nil, nil, nil)

	rows := table.Rows()
	view.SelectProcess(1, rows[1].Proc.Identity())

	memory.WriteProcesses([]procmodel.Process{
		{PID: 10, Started: 1, Name: "first", CPU: 1},
		{PID: 20, Started: 2, Name: "second", CPU: 99},
	})
	selected, ok := table.Selected()
	if !ok || selected.PID != 20 || view.Selected() != 0 {
		t.Fatalf("after refresh: process=%+v row=%d", selected, view.Selected())
	}

	view.SetSort(store.SortPID)
	selected, ok = table.Selected()
	if !ok || selected.PID != 20 || view.Selected() != 0 {
		t.Fatalf("after sort: process=%+v row=%d", selected, view.Selected())
	}

	view.SetOffset(1)
	table.theme = theme.Build(render.Caps{Glyphs: render.ASCII, Colors: render.Mono}, nil)
	program := reactea.New(table, reactea.WithSize(40, 3))
	_ = program.Init()
	_ = program.View()
	if view.Offset() != 0 {
		t.Fatalf("selected process remained outside the viewport at offset %d", view.Offset())
	}
}

func TestSelectionDoesNotAttachToAReusedPID(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	view := store.NewViewState()
	memory.WriteProcesses([]procmodel.Process{
		{PID: 10, Started: 1, CPU: 90},
		{PID: 20, Started: 2, CPU: 10},
	})
	table := New(memory, view, nil, nil, nil)
	rows := table.Rows()
	view.SelectProcess(0, rows[0].Proc.Identity())

	memory.WriteProcesses([]procmodel.Process{
		{PID: 10, Started: 3, CPU: 1},
		{PID: 20, Started: 2, CPU: 10},
	})
	selected, ok := table.Selected()
	if !ok || selected.PID != 20 {
		t.Fatalf("selection followed reused PID: %+v", selected)
	}
}
