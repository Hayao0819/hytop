// Package procmodel defines process snapshots and tree operations.
package procmodel

import (
	"slices"
	"sort"
)

// Process is one process snapshot; unavailable fields remain zero.
type Process struct {
	PID     int
	PPID    int
	PGID    int
	Started uint64

	Name    string
	Cmdline string
	Exe     string
	User    string
	UID     int64
	State   string
	Nice    int
	Threads int

	CPU   float64
	RSS   uint64
	VSZ   uint64
	FDs   int
	IORd  uint64
	IOWr  uint64
	GPUPc float64
	GPUMB uint64
	NPUPc float64

	Cgroup    string
	Unit      string
	Container string
	Runtime   string
	VM        string
	Ports     []int

	Kthread bool
}

type Identity struct {
	PID     int
	Started uint64
}

func (p *Process) Identity() Identity {
	if p == nil {
		return Identity{}
	}

	return Identity{PID: p.PID, Started: p.Started}
}

type Totals struct {
	Procs   int
	Threads int
	CPU     float64
	RSS     uint64
	IORd    uint64
	IOWr    uint64
}

// Snapshot is immutable after construction.
type Snapshot struct {
	byPID    map[int]*Process
	children map[int][]int
	roots    []int
	order    []int
}

// NewSnapshot promotes processes with missing parents to roots.
func NewSnapshot(procs []Process) *Snapshot {
	s := &Snapshot{
		byPID:    make(map[int]*Process, len(procs)),
		children: make(map[int][]int, len(procs)),
		order:    make([]int, 0, len(procs)),
	}

	for i := range procs {
		p := &procs[i]
		s.byPID[p.PID] = p
		s.order = append(s.order, p.PID)
	}

	for _, pid := range s.order {
		p := s.byPID[pid]

		if p.PPID == p.PID {
			continue
		}

		if _, ok := s.byPID[p.PPID]; ok {
			s.children[p.PPID] = append(s.children[p.PPID], pid)

			continue
		}

		s.roots = append(s.roots, pid)
	}

	for parent := range s.children {
		sort.Ints(s.children[parent])
	}

	s.adopt()

	sort.Ints(s.roots)

	return s
}

// adopt promotes components unreachable from any root, including PID-reuse cycles.
func (s *Snapshot) adopt() {
	reached := make(map[int]bool, len(s.order))

	var mark func(pid int)

	mark = func(pid int) {
		if reached[pid] {
			return
		}

		reached[pid] = true

		for _, child := range s.children[pid] {
			mark(child)
		}
	}

	for _, pid := range s.roots {
		mark(pid)
	}

	for _, pid := range s.order {
		if reached[pid] {
			continue
		}

		s.roots = append(s.roots, pid)

		mark(pid)
	}
}

func (s *Snapshot) Len() int { return len(s.order) }

func (s *Snapshot) PIDs() []int { return s.order }

func (s *Snapshot) Get(pid int) (*Process, bool) {
	p, ok := s.byPID[pid]

	return p, ok
}

func (s *Snapshot) Roots() []int { return s.roots }

func (s *Snapshot) Children(pid int) []int { return s.children[pid] }

// Descendants returns descendants once each and excludes pid from cycles.
func (s *Snapshot) Descendants(pid int) []int {
	var (
		found []int
		seen  = map[int]bool{pid: true}
		stack = slices.Clone(s.children[pid])
	)

	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if seen[next] {
			continue
		}

		seen[next] = true
		found = append(found, next)
		stack = append(stack, s.children[next]...)
	}

	sort.Ints(found)

	return found
}

// Ancestors walks toward a root and stops at cycles.
func (s *Snapshot) Ancestors(pid int) []int {
	var (
		found []int
		seen  = map[int]bool{pid: true}
	)

	for {
		p, ok := s.byPID[pid]
		if !ok || seen[p.PPID] {
			return found
		}

		parent, ok := s.byPID[p.PPID]
		if !ok {
			return found
		}

		found = append(found, parent.PID)
		seen[parent.PID] = true
		pid = parent.PID
	}
}

func (s *Snapshot) Siblings(pid int) []int {
	p, ok := s.byPID[pid]
	if !ok {
		return nil
	}

	var among []int
	if _, hasParent := s.byPID[p.PPID]; hasParent {
		among = s.children[p.PPID]
	} else {
		among = s.roots
	}

	found := make([]int, 0, len(among))

	for _, sibling := range among {
		if sibling != pid {
			found = append(found, sibling)
		}
	}

	return found
}

// Rollup sums pid and its descendants.
func (s *Snapshot) Rollup(pid int) Totals {
	var totals Totals

	for _, member := range append([]int{pid}, s.Descendants(pid)...) {
		p, ok := s.byPID[member]
		if !ok {
			continue
		}

		totals.Procs++
		totals.Threads += p.Threads
		totals.CPU += p.CPU
		totals.RSS += p.RSS
		totals.IORd += p.IORd
		totals.IOWr += p.IOWr
	}

	return totals
}
