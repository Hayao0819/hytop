package procmodel

import (
	"cmp"
	"slices"
	"strings"
)

// Container is a process-derived view of one container. Process is the source
// of truth: a runtime disappearing must not leave a stale second inventory.
type Container struct {
	Key     string
	Runtime string
	ID      string
	Totals  Totals
}

// Containers groups the current process snapshot by its normalized container
// key. The returned slice is detached from the snapshot and safe to retain.
func (s *Snapshot) Containers() []Container {
	byKey := map[string]*Container{}

	for _, pid := range s.order {
		p := s.byPID[pid]
		if p.Container == "" {
			continue
		}

		container := byKey[p.Container]
		if container == nil {
			runtime, id, _ := strings.Cut(p.Container, ":")
			container = &Container{Key: p.Container, Runtime: runtime, ID: id}
			byKey[p.Container] = container
		}

		container.Totals.Procs++
		container.Totals.Threads += p.Threads
		container.Totals.CPU += p.CPU
		container.Totals.RSS += p.RSS
		container.Totals.IORd += p.IORd
		container.Totals.IOWr += p.IOWr
	}

	containers := make([]Container, 0, len(byKey))
	for _, container := range byKey {
		containers = append(containers, *container)
	}

	slices.SortFunc(containers, func(a, b Container) int {
		return cmp.Compare(a.Key, b.Key)
	})

	return containers
}
