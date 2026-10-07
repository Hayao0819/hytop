package app_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func containerProcesses() []procmodel.Process {
	return []procmodel.Process{
		{PID: 101, Name: "dock-main", Cmdline: "dock-main", User: "root", CPU: 80, RSS: 20 << 20, Container: "docker:aaaaaaaaaaaa", Runtime: "docker"},
		{PID: 102, Name: "dock-worker", Cmdline: "dock-worker", User: "root", CPU: 5, RSS: 10 << 20, Container: "docker:aaaaaaaaaaaa", Runtime: "docker"},
		{PID: 201, Name: "pod-main", Cmdline: "pod-main", User: "root", CPU: 20, RSS: 30 << 20, Container: "podman:bbbbbbbbbbbb", Runtime: "podman"},
		{PID: 301, Name: "host-only", Cmdline: "host-only", User: "root", CPU: 10, RSS: 40 << 20},
	}
}

func TestContainersShowOneInventoryAndItsProcesses(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)
	memory.WriteProcesses(containerProcesses())
	mode(t, program, "Containers")

	got := plain(program)
	for _, want := range []string{"2 containers", "3 processes", "docker:aaaaaaaaaaaa", "podman:bbbbbbbbbbbb", "dock-main", "dock-worker"} {
		if !strings.Contains(got, want) {
			t.Fatalf("container screen is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "host-only") || strings.Contains(got, "pod-main") {
		t.Fatalf("the selected container's process pane was not isolated:\n%s", got)
	}

	press(t, program, "j")
	got = plain(program)
	if !strings.Contains(got, "pod-main") || strings.Contains(got, "dock-main") {
		t.Fatalf("moving the container selection did not update its processes:\n%s", got)
	}
}

func TestProcessesCanFilterToTheSelectedContainerAndBack(t *testing.T) {
	t.Parallel()

	program, memory, view := fixture(t)
	memory.WriteProcesses(containerProcesses())
	mode(t, program, "Processes")

	press(t, program, "C")
	if src, _ := view.Filter(); src != `container == "docker:aaaaaaaaaaaa"` {
		t.Fatalf("container filter = %q", src)
	}

	got := plain(program)
	if !strings.Contains(got, "dock-worker") || strings.Contains(got, "pod-main") || strings.Contains(got, "host-only") {
		t.Fatalf("container filter did not isolate the process table:\n%s", got)
	}

	press(t, program, "esc")
	if src, _ := view.Filter(); src != "" {
		t.Fatalf("esc did not restore the previous filter: %q", src)
	}
}
