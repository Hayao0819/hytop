package procmodel

type Tree struct {
	Process  *Process
	Children []*Tree
}

func (s *Snapshot) Forest(pids []int) []*Tree {
	selected := make(map[int]bool, len(pids))
	nodes := make(map[int]*Tree, len(pids))
	for _, pid := range pids {
		if process, ok := s.Get(pid); ok {
			selected[pid] = true
			nodes[pid] = &Tree{Process: process}
		}
	}

	roots := make([]*Tree, 0, len(nodes))
	for _, pid := range pids {
		node, ok := nodes[pid]
		if !ok {
			continue
		}
		parent := node.Process.PPID
		if selected[parent] && parent != pid {
			nodes[parent].Children = append(nodes[parent].Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	reached := make(map[int]bool, len(nodes))
	var mark func(*Tree)
	mark = func(node *Tree) {
		if reached[node.Process.PID] {
			return
		}
		reached[node.Process.PID] = true
		for _, child := range node.Children {
			mark(child)
		}
	}
	for _, root := range roots {
		mark(root)
	}
	for _, pid := range pids {
		if node, ok := nodes[pid]; ok && !reached[pid] {
			roots = append(roots, node)
			mark(node)
		}
	}

	return roots
}
