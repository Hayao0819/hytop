package store

import (
	"sync"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type SortKey string

const (
	SortCPU  SortKey = "cpu"
	SortMem  SortKey = "rss"
	SortPID  SortKey = "pid"
	SortName SortKey = "name"
)

// ViewState stores UI state independently of mounted components.
type ViewState struct {
	mu sync.RWMutex

	selected   int
	selectedID procmodel.Identity
	offset     int
	sortBy     SortKey
	descending bool
	filterSrc  string
	filterExpr filter.Expr
	stack      []string
	tree       bool
	collapsed  map[procmodel.Identity]bool

	serviceSearch string
	kernel        bool
	logs          bool
}

func NewViewState() *ViewState {
	return &ViewState{sortBy: SortCPU, descending: true, collapsed: map[procmodel.Identity]bool{}}
}

// Tree reports whether the process table is hierarchical.
func (v *ViewState) Tree() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.tree
}

func (v *ViewState) ToggleTree() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.tree = !v.tree
}

func (v *ViewState) SetTree(on bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.tree = on
}

func (v *ViewState) Collapsed(id procmodel.Identity) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.collapsed[id]
}

func (v *ViewState) ToggleCollapsed(id procmodel.Identity) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.collapsed[id] {
		delete(v.collapsed, id)

		return
	}

	v.collapsed[id] = true
}

func (v *ViewState) Selected() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.selected
}

func (v *ViewState) SetSelected(i int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.selected = max(0, i)
	v.selectedID = procmodel.Identity{}
}

func (v *ViewState) ResetSelection() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.resetSelection()
}

func (v *ViewState) resetSelection() {
	v.selected, v.selectedID, v.offset = 0, procmodel.Identity{}, 0
}

func (v *ViewState) SelectProcess(index int, id procmodel.Identity) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.selected = max(0, index)
	v.selectedID = id
}

func (v *ViewState) ReconcileProcesses(ids []procmodel.Identity) int {
	v.mu.Lock()
	defer v.mu.Unlock()

	if len(ids) == 0 {
		v.resetSelection()

		return 0
	}

	if v.selectedID.PID != 0 {
		for index, id := range ids {
			if id == v.selectedID {
				v.selected = index

				return index
			}
		}
	}

	v.selected = min(v.selected, len(ids)-1)
	v.selectedID = ids[v.selected]

	return v.selected
}

func (v *ViewState) PruneCollapsed(ids []procmodel.Identity) {
	live := make(map[procmodel.Identity]bool, len(ids))
	for _, id := range ids {
		live[id] = true
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	for id := range v.collapsed {
		if !live[id] {
			delete(v.collapsed, id)
		}
	}
}

func (v *ViewState) Offset() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.offset
}

func (v *ViewState) SetOffset(i int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.offset = max(0, i)
}

func (v *ViewState) Sort() (SortKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.sortBy, v.descending
}

// SetSortOrder sets the sort key and direction explicitly.
func (v *ViewState) SetSortOrder(by SortKey, descending bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.sortBy, v.descending = by, descending
}

// SetSort selects a key or reverses the current key's direction.
func (v *ViewState) SetSort(by SortKey) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.sortBy == by {
		v.descending = !v.descending

		return
	}

	v.sortBy, v.descending = by, true
}

func (v *ViewState) Filter() (string, filter.Expr) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.filterSrc, v.filterExpr
}

func (v *ViewState) SetFilter(src string, expr filter.Expr) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.filterSrc, v.filterExpr = src, expr
}

// Kernel reports whether kernel threads are visible.
func (v *ViewState) Kernel() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.kernel
}

func (v *ViewState) SetKernel(on bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.kernel = on
}

func (v *ViewState) ToggleKernel() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.kernel = !v.kernel
}

// Logs reports whether the process journal pane is visible.
func (v *ViewState) Logs() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.logs
}

func (v *ViewState) SetLogs(on bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.logs = on
}

func (v *ViewState) ToggleLogs() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.logs = !v.logs
}

// ServiceSearch returns the active unit-list search.
func (v *ViewState) ServiceSearch() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.serviceSearch
}

func (v *ViewState) SetServiceSearch(text string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.serviceSearch = text
}

// PushFilter preserves the previous filter for drill-down navigation.
func (v *ViewState) PushFilter(src string, expr filter.Expr) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stack = append(v.stack, v.filterSrc)
	v.filterSrc, v.filterExpr = src, expr
	v.resetSelection()
}

func (v *ViewState) PopFilter() (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if len(v.stack) == 0 {
		return "", false
	}

	previous := v.stack[len(v.stack)-1]
	v.stack = v.stack[:len(v.stack)-1]

	return previous, true
}
