package page

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

func moveCursor(msg tea.Msg, keys *keymap.Map, scope keymap.Scope, cursor *selection.Cursor, total int) bool {
	switch {
	case keys.Is(msg, scope, keymap.Down):
		cursor.Move(1, total)
	case keys.Is(msg, scope, keymap.Up):
		cursor.Move(-1, total)
	case keys.Is(msg, scope, keymap.Top):
		cursor.Top()
	case keys.Is(msg, scope, keymap.Bottom):
		cursor.Bottom(total)
	default:
		return false
	}

	return true
}
