package procmodel

import "slices"

// Column defines a process-table field shared by configuration and rendering.
type Column struct {
	Key   string
	Title string

	// Width is measured in cells; zero makes the last column flexible.
	Width int

	Right bool

	// Tree places hierarchy markers in this column.
	Tree bool

	// Sort names the corresponding sort key.
	Sort string
}

// Columns returns all columns in display order.
func Columns() []Column {
	return []Column{
		{Key: "pid", Title: "PID", Width: 7, Right: true, Sort: "pid"},
		{Key: "ppid", Title: "PPID", Width: 7, Right: true},
		{Key: "pgid", Title: "PGID", Width: 7, Right: true},
		{Key: "user", Title: "USER", Width: 10},
		{Key: "uid", Title: "UID", Width: 6, Right: true},
		{Key: "cpu", Title: "CPU%", Width: 6, Right: true, Sort: "cpu"},
		{Key: "rss", Title: "MEM", Width: 9, Right: true, Sort: "rss"},
		{Key: "vsz", Title: "VIRT", Width: 9, Right: true},
		{Key: "threads", Title: "THR", Width: 4, Right: true},
		{Key: "fds", Title: "FDS", Width: 5, Right: true},
		{Key: "nice", Title: "NI", Width: 4, Right: true},
		{Key: "state", Title: "S", Width: 2},
		{Key: "io.read", Title: "IO_R", Width: 9, Right: true},
		{Key: "io.write", Title: "IO_W", Width: 9, Right: true},
		{Key: "gpu.util", Title: "GPU%", Width: 6, Right: true},
		{Key: "gpu.mem", Title: "GPU MEM", Width: 9, Right: true},
		{Key: "npu.util", Title: "NPU%", Width: 6, Right: true},
		{Key: "unit", Title: "UNIT", Width: 24},
		{Key: "cgroup", Title: "CGROUP", Width: 30},
		{Key: "container", Title: "CONTAINER", Width: 16},
		{Key: "vm", Title: "VM", Width: 16},
		{Key: "exe", Title: "EXE", Width: 20},
		{Key: "name", Title: "NAME", Width: 16, Sort: "name"},
		{Key: "command", Title: "COMMAND", Tree: true, Sort: "name"},
		{Key: "cmdline", Title: "CMDLINE", Tree: true, Sort: "name"},
	}
}

// DefaultColumns returns the built-in process-table layout.
func DefaultColumns() []string {
	return []string{"pid", "user", "cpu", "rss", "threads", "state", "command"}
}

// ColumnKeys returns every configurable column name.
func ColumnKeys() []string {
	keys := make([]string, 0, len(Columns()))
	for _, column := range Columns() {
		keys = append(keys, column.Key)
	}

	return keys
}

// ColumnByKey resolves a configurable column name.
func ColumnByKey(key string) (Column, bool) {
	for _, column := range Columns() {
		if column.Key == key {
			return column, true
		}
	}

	return Column{}, false
}

// ResolveColumns validates and resolves configured column names.
func ResolveColumns(keys []string) ([]Column, error) {
	if len(keys) == 0 {
		keys = DefaultColumns()
	}

	resolved := make([]Column, 0, len(keys))

	for _, key := range keys {
		column, ok := ColumnByKey(key)
		if !ok {
			return nil, &UnknownColumnError{Key: key}
		}

		resolved = append(resolved, column)
	}

	// Only the final column may consume remaining width.
	for i := range resolved[:max(0, len(resolved)-1)] {
		if resolved[i].Width == 0 {
			resolved[i].Width = 20
		}
	}

	return resolved, nil
}

// UnknownColumnError reports an invalid configured column.
type UnknownColumnError struct{ Key string }

func (e *UnknownColumnError) Error() string {
	known := ColumnKeys()
	slices.Sort(known)

	return e.Key + " is not a column; there is " + join(known)
}

func join(values []string) string {
	out := ""

	for i, value := range values {
		switch {
		case i == 0:
		case i == len(values)-1:
			out += " and "
		default:
			out += ", "
		}

		out += value
	}

	return out
}
