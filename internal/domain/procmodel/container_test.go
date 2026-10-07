package procmodel_test

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func TestContainersRollUpTheProcessSnapshot(t *testing.T) {
	t.Parallel()

	snapshot := procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1, Container: "docker:aaaa", Runtime: "docker", CPU: 2, RSS: 10, Threads: 2},
		{PID: 2, Container: "docker:aaaa", Runtime: "docker", CPU: 3, RSS: 20, Threads: 4},
		{PID: 3, Container: "podman:bbbb", Runtime: "podman", CPU: 9, RSS: 30, Threads: 1},
		{PID: 4, CPU: 100},
	})

	got := snapshot.Containers()
	if len(got) != 2 || got[0].Key != "docker:aaaa" || got[0].Totals.Procs != 2 || got[0].Totals.CPU != 5 {
		t.Fatalf("Containers() = %+v", got)
	}
}
