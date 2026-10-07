package page

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// DeleteChoice is returned only after the destructive action is confirmed.
type DeleteChoice struct{ Path string }

type deleteDialog struct {
	reactea.BasicComponent

	theme *theme.Theme
	keys  *keymap.Map
	entry diskmodel.Entry
}

func newDeleteDialog(t *theme.Theme, keys *keymap.Map, entry diskmodel.Entry) *deleteDialog {
	return &deleteDialog{theme: t, keys: keys, entry: entry}
}

func (d *deleteDialog) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case d.keys.Is(msg, keymap.DeleteConfirm, keymap.Cancel):
		return modal.Dismiss(ctx)
	case d.keys.Is(msg, keymap.DeleteConfirm, keymap.Apply):
		return modal.Return(ctx, DeleteChoice{Path: d.entry.Path})
	}

	return nil
}

func (d *deleteDialog) Render(ctx *reactea.Ctx) string {
	inner := dialog.InnerWidth(ctx)
	clip := lipgloss.NewStyle().MaxWidth(inner)
	dim := d.theme.Style(theme.Dim)
	kind := "file"
	if d.entry.Dir {
		kind = "directory and everything inside it"
	}

	lines := []string{
		d.theme.Style(theme.Critical).Bold(true).Width(inner).Render(" Delete permanently?"),
		"",
		clip.Render(" " + safe.Text(d.entry.Path)),
		"",
		clip.Render(" " + dim.Render(fmt.Sprintf("This %s cannot be recovered by hytop.", kind))),
		clip.Render(" " + dim.Render(fmt.Sprintf("Size: %s   Items: %d", size(d.entry.Size), d.entry.Items))),
		"",
		clip.Render(" " + dim.Render(line(d.keys.Hints(keymap.DeleteConfirm)))),
	}

	return dialog.Frame(ctx, d.theme.Colour(theme.Critical), strings.Join(lines, "\n"))
}
