package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

type scrollView struct {
	reactea.BasicComponent

	keys   *keymap.Map
	scope  keymap.Scope
	offset int
}

func (s *scrollView) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case s.keys.Is(msg, s.scope, keymap.Cancel):
		return modal.Dismiss(ctx)
	case s.keys.Is(msg, s.scope, keymap.Down):
		s.offset++
	case s.keys.Is(msg, s.scope, keymap.Up):
		s.offset = max(0, s.offset-1)
	}

	return nil
}

func (s *scrollView) window(lines []string, height int) []string {
	start, end := selection.Window(s.offset, height, len(lines))
	s.offset = start

	return lines[start:end]
}

func (s *scrollView) footer(total, visible int) string {
	close := s.first(keymap.Cancel) + " close "
	if total <= visible {
		return " " + close
	}

	start, end := 0, 0
	if visible > 0 {
		start = s.offset + 1
		end = s.offset + visible
	}

	return fmt.Sprintf(" %d–%d / %d   %s/%s scroll   %s",
		start, end, total, s.first(keymap.Down), s.first(keymap.Up), close)
}

func (s *scrollView) first(action keymap.Action) string {
	if keys := s.keys.Keys(s.scope, action); len(keys) > 0 {
		return keys[0]
	}

	return "—"
}
