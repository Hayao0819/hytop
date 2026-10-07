package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
)

// help derives its labels from the active keymap.
type help struct {
	reactea.BasicComponent

	keys   *keymap.Map
	offset int
}

func newHelp(keys *keymap.Map) *help { return &help{keys: keys} }

func (h *help) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case h.keys.Is(msg, keymap.HelpScreen, keymap.Cancel):
		return modal.Dismiss(ctx)

	case h.keys.Is(msg, keymap.HelpScreen, keymap.Down):
		h.offset++

	case h.keys.Is(msg, keymap.HelpScreen, keymap.Up):
		h.offset = max(0, h.offset-1)
	}

	return nil
}

func (h *help) Render(ctx *reactea.Ctx) string {
	var (
		inner = dialog.InnerWidth(ctx)
		clip  = lipgloss.NewStyle().MaxWidth(inner)
		title = lipgloss.NewStyle().Bold(true)
		key   = lipgloss.NewStyle().Bold(true)
		dim   = lipgloss.NewStyle().Faint(true)
		lines = []string{title.Width(inner).Render(" hytop  keys ")}
	)

	for _, section := range h.keys.Sections() {
		lines = append(lines, "", clip.Render(" "+dim.Render(section.Title)))

		for _, hint := range section.Hints {
			lines = append(lines, clip.Render(" "+key.Render(pad(hint.Key, 18))+hint.What))
		}
	}

	lines = append(lines,
		"",
		clip.Render(" "+dim.Render("fields:    "+strings.Join(filter.Fields(), " "))),
		clip.Render(" "+dim.Render("relations: children descendants subtree ancestors siblings")),
		clip.Render(" "+dim.Render("operators: == != > < >= <= ~ ^= and or not")),
	)

	// Keep the title outside the scrolling region.
	bodyHeight := max(0, ctx.Height()-4)
	body := scroll(lines[1:], &h.offset, bodyHeight)
	position := fmt.Sprintf(" %d–%d / %d   j/k scroll   esc close ",
		min(h.offset+1, len(lines)-1), min(h.offset+len(body), len(lines)-1), len(lines)-1)
	body = append(body, dim.Width(inner).Align(lipgloss.Right).Render(position))

	return dialog.Frame(ctx, nil, strings.Join(append(lines[:1], body...), "\n"))
}

func scroll(lines []string, offset *int, height int) []string {
	*offset = max(0, min(*offset, len(lines)-height))

	if *offset >= len(lines) {
		return nil
	}

	return lines[*offset:min(*offset+height, len(lines))]
}

func pad(s string, width int) string {
	return uirender.Left(s, width)
}
