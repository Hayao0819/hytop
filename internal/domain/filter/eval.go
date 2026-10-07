package filter

import (
	"slices"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type set map[int]struct{}

func (s set) add(pid int) { s[pid] = struct{}{} }

func (s set) has(pid int) bool {
	_, ok := s[pid]

	return ok
}

type scope struct {
	snapshot *procmodel.Snapshot
	err      error
}

func Select(e Expr, snapshot *procmodel.Snapshot) ([]int, error) {
	if e == nil {
		return slices.Sorted(slices.Values(snapshot.PIDs())), nil
	}

	s := &scope{snapshot: snapshot}
	matched := e.eval(s)
	if s.err != nil {
		return nil, s.err
	}

	pids := make([]int, 0, len(matched))
	for pid := range matched {
		pids = append(pids, pid)
	}

	slices.Sort(pids)

	return pids, nil
}

func (e *andExpr) eval(s *scope) set {
	var (
		left  = e.left.eval(s)
		right = e.right.eval(s)
	)

	if len(right) < len(left) {
		left, right = right, left
	}

	both := make(set, len(left))

	for pid := range left {
		if right.has(pid) {
			both.add(pid)
		}
	}

	return both
}

func (e *orExpr) eval(s *scope) set {
	either := e.left.eval(s)

	for pid := range e.right.eval(s) {
		either.add(pid)
	}

	return either
}

func (e *notExpr) eval(s *scope) set {
	inner := e.inner.eval(s)

	rest := make(set, s.snapshot.Len()-len(inner))

	for _, pid := range s.snapshot.PIDs() {
		if !inner.has(pid) {
			rest.add(pid)
		}
	}

	return rest
}

func (e *relExpr) eval(s *scope) set {
	var (
		from    = e.inner.eval(s)
		related = make(set, len(from))
	)

	for pid := range from {
		switch e.kind {
		case relChildren:
			addAll(related, s.snapshot.Children(pid))

		case relDescendants:
			addAll(related, s.snapshot.Descendants(pid))

		case relSubtree:
			related.add(pid)
			addAll(related, s.snapshot.Descendants(pid))

		case relAncestors:
			addAll(related, s.snapshot.Ancestors(pid))

		case relSiblings:
			addAll(related, s.snapshot.Siblings(pid))
		}
	}

	return related
}

func (e *compareExpr) eval(s *scope) set {
	matched := make(set)

	for _, pid := range s.snapshot.PIDs() {
		p, ok := s.snapshot.Get(pid)
		if ok && e.matches(p) {
			matched.add(pid)
		}
	}

	return matched
}

func (e *truthExpr) eval(s *scope) set {
	matched := make(set)

	for _, pid := range s.snapshot.PIDs() {
		p, ok := s.snapshot.Get(pid)
		if ok && e.field.truth(p) {
			matched.add(pid)
		}
	}

	return matched
}

func addAll(s set, pids []int) {
	for _, pid := range pids {
		s.add(pid)
	}
}
