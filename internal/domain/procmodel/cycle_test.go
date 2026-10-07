package procmodel_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func cycle() []procmodel.Process {
	return []procmodel.Process{
		{PID: 1, PPID: 2, Name: "a", Threads: 1},
		{PID: 2, PPID: 1, Name: "b", Threads: 1},
		{PID: 3, PPID: 2, Name: "c", Threads: 1},
	}
}

func within(t *testing.T, what string, run func()) {
	t.Helper()

	done := make(chan struct{})

	go func() {
		defer close(done)

		run()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return: it is walking a PPID cycle for ever", what)
	}
}

func TestDescendantsSurvivesAPPIDCycle(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot(cycle())

	var found []int

	within(t, "Descendants", func() { found = snapshot.Descendants(1) })

	if len(found) != 2 || found[0] != 2 || found[1] != 3 {
		t.Errorf("Descendants(1) = %v, want [2 3]", found)
	}
}

func TestRollupCountsEachProcessOnceAcrossACycle(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot(cycle())

	var totals procmodel.Totals

	within(t, "Rollup", func() { totals = snapshot.Rollup(1) })

	if totals.Procs != 3 {
		t.Errorf("Rollup(1).Procs = %d, want 3 — every process once", totals.Procs)
	}
}

func TestNothingFallsOutOfTheTreeBecauseOfACycle(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot(cycle())

	if len(snapshot.Roots()) == 0 {
		t.Fatal("a cycle left the snapshot with no roots at all")
	}

	reached := map[int]bool{}

	var mark func(pid int)

	mark = func(pid int) {
		if reached[pid] {
			return
		}

		reached[pid] = true

		for _, child := range snapshot.Children(pid) {
			mark(child)
		}
	}

	within(t, "walking from the roots", func() {
		for _, root := range snapshot.Roots() {
			mark(root)
		}
	})

	for _, pid := range snapshot.PIDs() {
		if !reached[pid] {
			t.Errorf("pid %d is in the snapshot but no root reaches it", pid)
		}
	}
}

func TestDescendantsStillWalksAnOrdinaryTree(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1, PPID: 0},
		{PID: 2, PPID: 1},
		{PID: 3, PPID: 2},
		{PID: 4, PPID: 1},
	})

	found := snapshot.Descendants(1)
	if len(found) != 3 || found[0] != 2 || found[1] != 3 || found[2] != 4 {
		t.Errorf("Descendants(1) = %v, want [2 3 4]", found)
	}
}
