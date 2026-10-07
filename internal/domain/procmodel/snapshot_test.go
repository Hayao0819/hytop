package procmodel_test

import (
	"slices"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

// tree is
//
//	1 systemd
//	├── 10 sshd
//	│   ├── 11 bash
//	│   └── 12 bash
//	│       └── 13 vim
//	└── 20 firefox
//
// plus 99, whose parent is not in the reading.
func tree() *procmodel.Snapshot {
	return procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1, PPID: 0, Name: "systemd", Threads: 1, CPU: 0.1, RSS: 10},
		{PID: 10, PPID: 1, Name: "sshd", Threads: 1, CPU: 0.2, RSS: 20},
		{PID: 11, PPID: 10, Name: "bash", Threads: 1, CPU: 0.3, RSS: 30},
		{PID: 12, PPID: 10, Name: "bash", Threads: 1, CPU: 0.4, RSS: 40},
		{PID: 13, PPID: 12, Name: "vim", Threads: 2, CPU: 0.5, RSS: 50},
		{PID: 20, PPID: 1, Name: "firefox", Threads: 8, CPU: 12, RSS: 60},
		{PID: 99, PPID: 5000, Name: "orphan", Threads: 1, CPU: 1, RSS: 70},
	})
}

func TestChildrenAndDescendants(t *testing.T) {
	t.Parallel()

	s := tree()

	if got := s.Children(10); !slices.Equal(got, []int{11, 12}) {
		t.Errorf("Children(10) = %v", got)
	}

	if got := s.Children(11); got != nil {
		t.Errorf("Children(11) = %v, want none", got)
	}

	if got := s.Descendants(10); !slices.Equal(got, []int{11, 12, 13}) {
		t.Errorf("Descendants(10) = %v", got)
	}

	if got := s.Descendants(1); !slices.Equal(got, []int{10, 11, 12, 13, 20}) {
		t.Errorf("Descendants(1) = %v", got)
	}
}

func TestAncestors(t *testing.T) {
	t.Parallel()

	s := tree()

	if got := s.Ancestors(13); !slices.Equal(got, []int{12, 10, 1}) {
		t.Errorf("Ancestors(13) = %v", got)
	}

	if got := s.Ancestors(1); got != nil {
		t.Errorf("Ancestors(1) = %v, want none", got)
	}
}

func TestSiblings(t *testing.T) {
	t.Parallel()

	s := tree()

	if got := s.Siblings(11); !slices.Equal(got, []int{12}) {
		t.Errorf("Siblings(11) = %v", got)
	}

	if got := s.Siblings(10); !slices.Equal(got, []int{20}) {
		t.Errorf("Siblings(10) = %v", got)
	}
}

func TestAProcessWhoseParentIsGoneBecomesARoot(t *testing.T) {
	t.Parallel()

	s := tree()

	if got := s.Roots(); !slices.Equal(got, []int{1, 99}) {
		t.Errorf("Roots = %v, want 1 and the orphan", got)
	}

	if s.Len() != 7 {
		t.Errorf("Len = %d, want every process kept", s.Len())
	}

	if got := s.Siblings(99); !slices.Equal(got, []int{1}) {
		t.Errorf("Siblings(99) = %v", got)
	}
}

func TestRollupAddsTheSubtreeToItsParent(t *testing.T) {
	t.Parallel()

	s := tree()

	got := s.Rollup(10)

	want := procmodel.Totals{Procs: 4, Threads: 5, CPU: 1.4, RSS: 140}
	if got.Procs != want.Procs || got.Threads != want.Threads || got.RSS != want.RSS {
		t.Errorf("Rollup(10) = %+v, want %+v", got, want)
	}

	if got.CPU < 1.39 || got.CPU > 1.41 {
		t.Errorf("Rollup(10).CPU = %v, want about 1.4", got.CPU)
	}

	if leaf := s.Rollup(13); leaf.Procs != 1 || leaf.RSS != 50 {
		t.Errorf("Rollup(13) = %+v, want just itself", leaf)
	}
}

func TestACycleDoesNotHangTheWalk(t *testing.T) {
	t.Parallel()

	s := procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1, PPID: 2},
		{PID: 2, PPID: 1},
	})

	if got := s.Ancestors(1); len(got) > 2 {
		t.Errorf("Ancestors on a cycle = %v", got)
	}
}
