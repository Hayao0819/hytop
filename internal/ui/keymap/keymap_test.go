package keymap_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Hayao0819/hytop/internal/ui/keymap"
)

func press(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	default:
		return tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
}

func TestTheDefaultTableHasNoClashes(t *testing.T) {
	t.Parallel()

	if _, err := keymap.Default().Rebind(map[string][]string{}); err != nil {
		t.Fatal(err)
	}

	if _, err := keymap.Default().Rebind(map[string][]string{"processes.filter": {"t"}}); err == nil {
		t.Error("two actions on t in the same scope should be refused")
	}
}

func TestAScopeInheritsFromItsParent(t *testing.T) {
	t.Parallel()

	keys := keymap.Default()

	if !keys.Is(press("z"), keymap.ProcessesTree, keymap.Fold) {
		t.Error("the tree scope should own z")
	}

	if !keys.Is(press("/"), keymap.ProcessesTree, keymap.Filter) {
		t.Error("the tree scope should inherit the filter key")
	}

	if keys.Is(press("z"), keymap.Processes, keymap.Fold) {
		t.Error("the flat list has no fold; a child's key must not reach the parent")
	}
}

func TestAScopeWithNoParentIsSealed(t *testing.T) {
	t.Parallel()

	keys := keymap.Default()

	if keys.Is(press("q"), keymap.Filtering, keymap.Quit) {
		t.Error("q is a character while the filter bar is open, not a quit")
	}

	if keys.Is(press("q"), keymap.Kill, keymap.Quit) {
		t.Error("the dialog answers q itself; the global quit must not fire under it")
	}

	if !keys.Is(press("q"), keymap.Kill, keymap.Cancel) {
		t.Error("q should cancel the dialog")
	}
}

func TestTheNearestScopeOwnsAnAction(t *testing.T) {
	t.Parallel()

	keys, err := keymap.Default().Rebind(map[string][]string{"services.down": {"n"}})
	if err != nil {
		t.Fatal(err)
	}

	if !keys.Is(press("n"), keymap.Services, keymap.Down) {
		t.Error("the rebound key should work")
	}

	if keys.Is(press("j"), keymap.Services, keymap.Down) {
		t.Error("the old key should be gone, not kept alongside the new one")
	}
}

func TestIndexSaysWhichKeyOfARow(t *testing.T) {
	t.Parallel()

	keys := keymap.Default()

	if got := keys.Index(press("3"), keymap.Global, keymap.Mode); got != 2 {
		t.Errorf("Index(3) = %d, want 2", got)
	}

	if got := keys.Index(press("x"), keymap.Global, keymap.Mode); got != -1 {
		t.Errorf("Index(x) = %d, want -1", got)
	}
}

func TestHintsMergeWhatSharesALabel(t *testing.T) {
	t.Parallel()

	hints := keymap.Default().Hints(keymap.Processes)

	var move, sort string

	for _, hint := range hints {
		switch hint.What {
		case "move":
			move = hint.Key
		case "sort":
			sort = hint.Key
		}
	}

	if move != "j k" {
		t.Errorf("move = %q, want the two keys on one entry", move)
	}

	if sort != "c m p n" {
		t.Errorf("sort = %q, want all four columns on one entry", sort)
	}
}

func TestHintsPutThePagesOwnKeysFirst(t *testing.T) {
	t.Parallel()

	hints := keymap.Default().Hints(keymap.Graphs)
	if len(hints) == 0 {
		t.Fatal("no hints")
	}

	if hints[0].What != "device" {
		t.Errorf("the line starts with %q; the page's own keys should lead", hints[0].What)
	}
}

func TestHintsLeaveOutWhatIsHidden(t *testing.T) {
	t.Parallel()

	for _, hint := range keymap.Default().Hints(keymap.Graphs) {
		if strings.Contains(hint.Key, "ctrl+c") || strings.Contains(hint.Key, "shift+tab") {
			t.Errorf("%q should not be on the key line", hint.Key)
		}
	}
}

func TestTheHelpListsEveryScope(t *testing.T) {
	t.Parallel()

	sections := keymap.Default().Sections()

	titles := make([]string, 0, len(sections))

	var found bool

	for _, section := range sections {
		titles = append(titles, section.Title)

		for _, hint := range section.Hints {
			if hint.Key == "ctrl+c" {
				found = true
			}
		}
	}

	if len(sections) < 8 {
		t.Errorf("the help has only %v", titles)
	}

	if !found {
		t.Error("ctrl+c is off the key line but belongs in the help")
	}
}

func TestRebindingRefusesWhatItCannotHonour(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string][]string{
		"an unknown binding": {"processes.explode": {"x"}},
		"a missing scope":    {"quit": {"x"}},
		"nothing to press":   {"global.quit": {}},
	}

	for what, overrides := range cases {
		t.Run(what, func(t *testing.T) {
			t.Parallel()

			if _, err := keymap.Default().Rebind(overrides); err == nil {
				t.Errorf("%s should be refused", what)
			}
		})
	}
}

func TestRebindingLeavesTheRestAlone(t *testing.T) {
	t.Parallel()

	keys, err := keymap.Default().Rebind(map[string][]string{"global.quit": {"Q"}})
	if err != nil {
		t.Fatal(err)
	}

	if !keys.Is(press("Q"), keymap.Graphs, keymap.Quit) {
		t.Error("the new quit key does not work")
	}

	if !keys.Is(press("j"), keymap.Graphs, keymap.Down) {
		t.Error("rebinding one key disturbed another")
	}

	if !keymap.Default().Is(press("q"), keymap.Graphs, keymap.Quit) {
		t.Error("Rebind changed the map it was called on")
	}
}

func TestTheBuiltInTableValidates(t *testing.T) {
	t.Parallel()

	if err := keymap.Default().Validate(); err != nil {
		t.Fatal(err)
	}
}
