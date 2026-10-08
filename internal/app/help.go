package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
)

// help derives its labels from the active keymap.
type help struct {
	scrollView
}

func newHelp(keys *keymap.Map) *help {
	return &help{scrollView: scrollView{keys: keys, scope: keymap.HelpScreen}}
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
	body := h.window(lines[1:], bodyHeight)
	position := h.footer(len(lines)-1, len(body))
	body = append(body, dim.Width(inner).Align(lipgloss.Right).Render(position))

	return dialog.Frame(ctx, nil, strings.Join(append(lines[:1], body...), "\n"))
}

func pad(s string, width int) string {
	return uirender.Left(s, width)
}
