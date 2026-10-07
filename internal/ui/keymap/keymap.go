// Package keymap maps configurable key sequences to application actions.
package keymap

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/errors"
)

// Action identifies a configurable operation.
type Action string

const (
	Mode      Action = "mode"
	NextMode  Action = "next-mode"
	PrevMode  Action = "prev-mode"
	Help      Action = "help"
	About     Action = "about"
	Quit      Action = "quit"
	ForceQuit Action = "force-quit"

	Down     Action = "down"
	Up       Action = "up"
	Top      Action = "top"
	Bottom   Action = "bottom"
	HalfDown Action = "half-down"
	HalfUp   Action = "half-up"
	Open     Action = "open"
	Back     Action = "back"

	PerCore Action = "per-core"

	Filter          Action = "filter"
	FilterContainer Action = "filter-container"
	Signal          Action = "signal"
	ToggleTree      Action = "toggle-tree"
	ToggleKernel    Action = "toggle-kernel"
	ToggleLogs      Action = "toggle-logs"
	Fold            Action = "fold"
	Drill           Action = "drill"
	Apply           Action = "apply"
	Cancel          Action = "cancel"

	SortCPU  Action = "sort-cpu"
	SortMem  Action = "sort-mem"
	SortPID  Action = "sort-pid"
	SortName Action = "sort-name"

	Less  Action = "less"
	More  Action = "more"
	Write Action = "write"

	NextPane  Action = "next-pane"
	PrevPane  Action = "prev-pane"
	EnterPane Action = "enter-pane"
	LeavePane Action = "leave-pane"
	Rescan    Action = "rescan"
	Remeasure Action = "remeasure"
	Delete    Action = "delete"
	Elevate   Action = "elevate"
)

// Scope identifies a screen and inherits bindings from its parent.
type Scope string

const (
	Global             Scope = "global"
	Graphs             Scope = "graphs"
	CPU                Scope = "cpu"
	Processes          Scope = "processes"
	ProcessesTree      Scope = "processes-tree"
	Containers         Scope = "containers"
	ContainerProcesses Scope = "container-processes"
	Filtering          Scope = "filtering"
	Services           Scope = "services"
	Unit               Scope = "unit"
	Settings           Scope = "settings"
	Disk               Scope = "disk"
	Drives             Scope = "drives"
	Usage              Scope = "usage"
	Kill               Scope = "kill"
	DeleteConfirm      Scope = "delete-confirm"
	Auth               Scope = "auth"
	HelpScreen         Scope = "help"
	AboutScreen        Scope = "about"
)

// An empty parent prevents modal and text-input scopes from inheriting keys.
var scopes = []struct {
	scope  Scope
	parent Scope
	title  string
}{
	{Global, "", "Everywhere"},
	{Graphs, Global, "Graphs"},
	{CPU, Graphs, "Graphs, CPU"},
	{Processes, Global, "Processes"},
	{ProcessesTree, Processes, "Processes, tree"},
	{Containers, Global, "Containers"},
	{ContainerProcesses, Containers, "Containers, processes"},
	{Filtering, "", "Processes, filtering"},
	{Services, Global, "Services"},
	{Unit, Global, "Services, one unit"},
	{Settings, Global, "Settings"},
	{Disk, Global, "Disk"},
	{Drives, Disk, "Disk, drives"},
	{Usage, Disk, "Disk, usage"},
	{Kill, "", "Signal dialog"},
	{DeleteConfirm, "", "Delete confirmation"},
	{Auth, "", "Administrator authentication"},
	{HelpScreen, "", "Help"},
	{AboutScreen, "", "About"},
}

// Binding maps keys to an action in one scope.
type Binding struct {
	Scope  Scope
	Action Action
	Keys   []string

	// Short groups bindings under one compact hint label.
	What  string
	Short string

	// Hidden omits a binding from the key line but not the help screen.
	Hidden bool
}

type Map struct {
	bindings []Binding
	byScope  map[Scope][]Binding
	parent   map[Scope]Scope
	title    map[Scope]string
}

func newMap(bindings []Binding) *Map {
	m := &Map{
		bindings: bindings,
		byScope:  make(map[Scope][]Binding, len(scopes)),
		parent:   make(map[Scope]Scope, len(scopes)),
		title:    make(map[Scope]string, len(scopes)),
	}

	for _, s := range scopes {
		m.parent[s.scope], m.title[s.scope] = s.parent, s.title
	}

	for _, binding := range bindings {
		m.byScope[binding.Scope] = append(m.byScope[binding.Scope], binding)
	}

	return m
}

// Is reports whether msg invokes action in scope or an inherited scope.
func (m *Map) Is(msg tea.Msg, scope Scope, action Action) bool {
	return m.Index(msg, scope, action) >= 0
}

// Index returns the matching key index for action, or -1.
func (m *Map) Index(msg tea.Msg, scope Scope, action Action) int {
	for at := scope; at != ""; at = m.parent[at] {
		for _, binding := range m.byScope[at] {
			if binding.Action != action {
				continue
			}

			for i, key := range binding.Keys {
				if reactea.Key(msg, key) {
					return i
				}
			}

			// A nearer scope shadows the same action in its ancestors.
			return -1
		}
	}

	return -1
}

// Keys returns an action's nearest binding.
func (m *Map) Keys(scope Scope, action Action) []string {
	for at := scope; at != ""; at = m.parent[at] {
		for _, binding := range m.byScope[at] {
			if binding.Action == action {
				return binding.Keys
			}
		}
	}

	return nil
}

// Hints returns compact bindings for a scope and its ancestors.
func (m *Map) Hints(scope Scope, excluded ...Action) []Hint {
	var (
		hints []Hint
		at    = map[string]int{}
		seen  = map[Action]bool{}
	)
	for _, action := range excluded {
		seen[action] = true
	}

	for scope := scope; scope != ""; scope = m.parent[scope] {
		for _, binding := range m.byScope[scope] {
			if binding.Hidden || binding.What == "" || seen[binding.Action] {
				continue
			}

			seen[binding.Action] = true

			label := binding.Short
			if label == "" {
				label = binding.What
			}

			if i, ok := at[label]; ok {
				hints[i].Key += " " + show(binding.Keys)

				continue
			}

			at[label] = len(hints)
			hints = append(hints, Hint{Key: show(binding.Keys), What: label})
		}
	}

	return hints
}

// Sections returns all bindings grouped for the help screen.
func (m *Map) Sections() []Section {
	sections := make([]Section, 0, len(scopes))

	for _, s := range scopes {
		bindings := m.byScope[s.scope]
		if len(bindings) == 0 {
			continue
		}

		hints := make([]Hint, 0, len(bindings))

		for _, binding := range bindings {
			if binding.What == "" {
				continue
			}

			hints = append(hints, Hint{Key: join(binding.Keys), What: binding.What})
		}

		if len(hints) > 0 {
			sections = append(sections, Section{Title: s.title, Hints: hints})
		}
	}

	return sections
}

func show(keys []string) string {
	switch {
	case len(keys) == 0:
		return "—"
	case len(keys) > 3:
		return keys[0] + ".." + keys[len(keys)-1]
	default:
		return keys[0]
	}
}

func join(keys []string) string {
	if len(keys) > 3 {
		return keys[0] + ".." + keys[len(keys)-1]
	}

	return strings.Join(keys, " / ")
}

// Name returns the binding name used in configuration files.
func Name(scope Scope, action Action) string { return string(scope) + "." + string(action) }

// Rebind returns a validated map with named bindings replaced.
func (m *Map) Rebind(overrides map[string][]string) (*Map, error) {
	if len(overrides) == 0 {
		return m, m.Validate()
	}

	known := make(map[string]int, len(m.bindings))
	for i, binding := range m.bindings {
		known[Name(binding.Scope, binding.Action)] = i
	}

	bindings := slices.Clone(m.bindings)

	for name, keys := range overrides {
		at, ok := known[name]
		if !ok {
			return nil, errors.Newf("%s is not a binding; hytop keys lists them all", name)
		}

		if len(keys) == 0 {
			return nil, errors.Newf("%s is bound to nothing; remove the line to keep the default", name)
		}

		bindings[at].Keys = keys
	}

	rebound := newMap(bindings)

	if err := rebound.Validate(); err != nil {
		return nil, err
	}

	return rebound, nil
}

// Validate rejects keys assigned to different actions in overlapping scopes.
func (m *Map) Validate() error {
	for _, s := range scopes {
		taken := map[string]Action{}

		for at := s.scope; at != ""; at = m.parent[at] {
			for _, binding := range m.byScope[at] {
				for _, key := range binding.Keys {
					if other, ok := taken[key]; ok && other != binding.Action {
						return errors.Newf("in %s, %q is bound to both %s and %s",
							s.scope, key, other, binding.Action)
					}

					taken[key] = binding.Action
				}
			}
		}
	}

	return nil
}

// Names returns every binding name accepted in configuration files.
func (m *Map) Names() []string {
	names := make([]string, 0, len(m.bindings))
	for _, binding := range m.bindings {
		names = append(names, Name(binding.Scope, binding.Action))
	}

	return names
}

// Title returns the scope heading used by the help screen.
func (m *Map) Title(scope Scope) string { return m.title[scope] }

// All returns bindings in display order.
func (m *Map) All() []Binding { return m.bindings }
