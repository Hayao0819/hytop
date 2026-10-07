package procmodel_test

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func TestForestReparentsFilteredProcessesAndRetainsCycles(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1},
		{PID: 2, PPID: 1},
		{PID: 3, PPID: 2},
		{PID: 10, PPID: 11},
		{PID: 11, PPID: 10},
	})
	forest := snapshot.Forest([]int{3, 10, 11})
	if len(forest) != 2 || forest[0].Process.PID != 3 || forest[1].Process.PID != 10 {
		t.Fatalf("roots = %v", treePIDs(forest))
	}
	if len(forest[1].Children) != 1 || forest[1].Children[0].Process.PID != 11 {
		t.Fatalf("cycle was not retained from an adopted root: %v", treePIDs(forest))
	}
}

func treePIDs(nodes []*procmodel.Tree) []int {
	pids := make([]int, 0, len(nodes))
	for _, node := range nodes {
		pids = append(pids, node.Process.PID)
	}

	return pids
}

func TestEveryColumnUsesTheProcessFieldNamespace(t *testing.T) {
	t.Parallel()

	for _, column := range procmodel.Columns() {
		if _, ok := procmodel.LookupField(column.Key); !ok {
			t.Errorf("column %q has no process field", column.Key)
		}
	}
}
