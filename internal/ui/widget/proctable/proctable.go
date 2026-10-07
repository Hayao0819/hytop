// Package proctable draws the process table.
package proctable

import (
	"cmp"
	"os"
	"os/user"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type Widget struct {
	reactea.BasicComponent

	store   *store.Store
	view    *store.ViewState
	theme   *theme.Theme
	keys    *keymap.Map
	columns []procmodel.Column
	scope   keymap.Scope
	err     error

	self string
}

func New(
	s *store.Store, v *store.ViewState, t *theme.Theme, keys *keymap.Map, columns []procmodel.Column,
) *Widget {
	if len(columns) == 0 {
		columns, _ = procmodel.ResolveColumns(nil)
	}

	if keys == nil {
		keys = keymap.Default()
	}

	self := os.Getenv("USER")
	if u, err := user.Current(); err == nil {
		self = u.Username
	}

	return &Widget{store: s, view: v, theme: t, keys: keys, columns: columns, scope: keymap.Processes, self: self}
}

// WithScope sets the keymap scope used by the table.
func (w *Widget) WithScope(scope keymap.Scope) *Widget {
	w.scope = scope

	return w
}

// Row contains a process and its optional tree prefix.
type Row struct {
	Proc      *procmodel.Process
	Prefix    string
	Children  bool
	Collapsed bool
}

// Rows returns the currently visible process rows.
func (w *Widget) Rows() []Row {
	var (
		snapshot = w.store.Processes()
		_, expr  = w.view.Filter()
	)
	pids, err := filter.Select(expr, snapshot)
	w.err = err
	if err != nil {
		return nil
	}
	pids = w.visible(snapshot, pids)
	identities := make([]procmodel.Identity, 0, snapshot.Len())
	for _, pid := range snapshot.PIDs() {
		if proc, ok := snapshot.Get(pid); ok {
			identities = append(identities, proc.Identity())
		}
	}
	w.view.PruneCollapsed(identities)

	if w.view.Tree() {
		rows := w.tree(snapshot, pids)
		w.reconcile(rows)

		return rows
	}

	procs := make([]*procmodel.Process, 0, len(pids))

	for _, pid := range pids {
		if p, ok := snapshot.Get(pid); ok {
			procs = append(procs, p)
		}
	}

	w.sort(procs)

	rows := make([]Row, 0, len(procs))
	for _, p := range procs {
		rows = append(rows, Row{Proc: p})
	}
	w.reconcile(rows)

	return rows
}

func (w *Widget) reconcile(rows []Row) {
	ids := make([]procmodel.Identity, len(rows))
	for index, row := range rows {
		ids[index] = row.Proc.Identity()
	}

	w.view.ReconcileProcesses(ids)
}

func (w *Widget) Error() error { return w.err }

// visible applies the kernel-thread toggle without altering the user filter.
func (w *Widget) visible(snapshot *procmodel.Snapshot, pids []int) []int {
	if w.view.Kernel() {
		return pids
	}

	kept := make([]int, 0, len(pids))

	for _, pid := range pids {
		if p, ok := snapshot.Get(pid); ok && p.Kthread {
			continue
		}

		kept = append(kept, pid)
	}

	return kept
}

// tree promotes matches whose parents were filtered out to roots.
func (w *Widget) tree(snapshot *procmodel.Snapshot, pids []int) []Row {
	roots := snapshot.Forest(pids)
	w.sortTrees(roots)

	rows := make([]Row, 0, len(pids))

	var walk func(nodes []*procmodel.Tree, ancestors string, root bool)

	// Guard against PID-reuse cycles in parent relationships.
	drawn := make(map[int]bool, len(pids))

	walk = func(nodes []*procmodel.Tree, ancestors string, root bool) {
		for i, node := range nodes {
			p := node.Process
			if drawn[p.PID] {
				continue
			}

			drawn[p.PID] = true

			var (
				last      = i == len(nodes)-1
				kids      = node.Children
				collapsed = w.view.Collapsed(p.Identity())
				prefix    string
			)

			if !root {
				prefix = ancestors + branch(last)
			}

			rows = append(rows, Row{
				Proc:      p,
				Prefix:    prefix,
				Children:  len(kids) > 0,
				Collapsed: collapsed,
			})

			if len(kids) == 0 || collapsed {
				continue
			}

			next := ""
			if !root {
				next = ancestors + trail(last)
			}

			walk(kids, next, false)
		}
	}

	walk(roots, "", true)

	return rows
}

func branch(last bool) string {
	if last {
		return "└─"
	}

	return "├─"
}

func trail(last bool) string {
	if last {
		return "  "
	}

	return "│ "
}

func (w *Widget) sort(procs []*procmodel.Process) {
	by, descending := w.view.Sort()

	slices.SortStableFunc(procs, func(a, b *procmodel.Process) int {
		order := compare(a, b, by)
		if descending {
			return -order
		}

		return order
	})
}

func (w *Widget) sortTrees(nodes []*procmodel.Tree) {
	w.sortTreeNodes(nodes, make(map[int]bool, len(nodes)))
}

func (w *Widget) sortTreeNodes(nodes []*procmodel.Tree, seen map[int]bool) {
	by, descending := w.view.Sort()
	slices.SortStableFunc(nodes, func(a, b *procmodel.Tree) int {
		order := compare(a.Process, b.Process, by)
		if descending {
			return -order
		}

		return order
	})
	for _, node := range nodes {
		if seen[node.Process.PID] {
			continue
		}
		seen[node.Process.PID] = true
		w.sortTreeNodes(node.Children, seen)
	}
}

func (w *Widget) Selected() (*procmodel.Process, bool) {
	rows := w.Rows()
	if len(rows) == 0 {
		return nil, false
	}

	return rows[min(w.view.Selected(), len(rows)-1)].Proc, true
}

func compare(a, b *procmodel.Process, by store.SortKey) int {
	switch by {
	case store.SortMem:
		return cmp.Compare(a.RSS, b.RSS)
	case store.SortPID:
		return cmp.Compare(a.PID, b.PID)
	case store.SortName:
		return -cmp.Compare(a.Name, b.Name)
	default:
		return cmp.Compare(a.CPU, b.CPU)
	}
}

func (w *Widget) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if _, _, inside := reactea.Mouse(ctx, msg); inside && msg.Y > 0 {
			rows := w.Rows()
			w.selectRow(rows, w.view.Offset()+msg.Y-1)
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			w.move(ctx, -3)
		case tea.MouseWheelDown:
			w.move(ctx, 3)
		}

	case tea.KeyPressMsg:
		w.key(ctx, msg)
	}

	return nil
}

func (w *Widget) key(ctx *reactea.Ctx, msg tea.Msg) {
	_, height := ctx.Size()

	switch {
	case w.keys.Is(msg, w.scope, keymap.Down):
		w.move(ctx, 1)
	case w.keys.Is(msg, w.scope, keymap.Up):
		w.move(ctx, -1)
	case w.keys.Is(msg, w.scope, keymap.Top):
		w.selectRow(w.Rows(), 0)
	case w.keys.Is(msg, w.scope, keymap.Bottom):
		rows := w.Rows()
		w.selectRow(rows, len(rows)-1)
	case w.keys.Is(msg, w.scope, keymap.HalfDown):
		w.move(ctx, max(1, height/2))
	case w.keys.Is(msg, w.scope, keymap.HalfUp):
		w.move(ctx, -max(1, height/2))
	case w.keys.Is(msg, w.scope, keymap.SortCPU):
		w.view.SetSort(store.SortCPU)
	case w.keys.Is(msg, w.scope, keymap.SortMem):
		w.view.SetSort(store.SortMem)
	case w.keys.Is(msg, w.scope, keymap.SortPID):
		w.view.SetSort(store.SortPID)
	case w.keys.Is(msg, w.scope, keymap.SortName):
		w.view.SetSort(store.SortName)
	}

	w.scrollIntoView(ctx)
}

func (w *Widget) move(ctx *reactea.Ctx, by int) {
	rows := w.Rows()
	if len(rows) == 0 {
		return
	}

	w.selectRow(rows, min(max(w.view.Selected()+by, 0), len(rows)-1))
	w.scrollIntoView(ctx)
}

func (w *Widget) selectRow(rows []Row, index int) {
	if index < 0 || index >= len(rows) {
		return
	}

	w.view.SelectProcess(index, rows[index].Proc.Identity())
}

func (w *Widget) scrollIntoView(ctx *reactea.Ctx) {
	_, height := ctx.Size()

	visible := max(1, height-1)

	var (
		selected = w.view.Selected()
		offset   = w.view.Offset()
	)

	switch {
	case selected < offset:
		w.view.SetOffset(selected)
	case selected >= offset+visible:
		w.view.SetOffset(selected - visible + 1)
	}
}
