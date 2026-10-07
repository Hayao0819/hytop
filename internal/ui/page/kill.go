package page

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// Signals are the ones worth a key. TERM first, because asking is what should
// be easy and KILL is what should take a deliberate move.
type processSignal int

type signalChoice struct {
	Name   string
	Signal processSignal
	What   string
}

// Choice is what the dialog answers with.
type Choice struct {
	Identity procmodel.Identity
	Signal   processSignal
	Name     string
}

type signalTarget struct {
	identity procmodel.Identity
	handle   int
}

type killDialog struct {
	reactea.BasicComponent

	theme    *theme.Theme
	keys     *keymap.Map
	identity procmodel.Identity
	command  string
	selected int
}

func newKillDialog(t *theme.Theme, keys *keymap.Map, identity procmodel.Identity, command string) *killDialog {
	return &killDialog{theme: t, keys: keys, identity: identity, command: command}
}

func (d *killDialog) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case d.keys.Is(msg, keymap.Kill, keymap.Cancel):
		return modal.Dismiss(ctx)

	case d.keys.Is(msg, keymap.Kill, keymap.Down):
		d.selected = min(d.selected+1, len(Signals)-1)

	case d.keys.Is(msg, keymap.Kill, keymap.Up):
		d.selected = max(d.selected-1, 0)

	case d.keys.Is(msg, keymap.Kill, keymap.Apply):
		chosen := Signals[d.selected]

		return modal.Return(ctx, Choice{Identity: d.identity, Signal: chosen.Signal, Name: chosen.Name})
	}

	return nil
}

func (d *killDialog) Render(ctx *reactea.Ctx) string {
	var (
		inner = dialog.InnerWidth(ctx)
		clip  = lipgloss.NewStyle().MaxWidth(inner)
		dim   = d.theme.Style(theme.Dim)
		lines = []string{
			d.theme.Style(theme.Heading).Width(inner).Render(fmt.Sprintf(" signal %d", d.identity.PID)),
			clip.Render(" " + dim.Render(safe.Text(d.command))),
			"",
		}
	)

	for i, signal := range Signals {
		line := fmt.Sprintf("  %-9s %s", signal.Name, signal.What)

		if i == d.selected {
			style := d.theme.Style(theme.Selected)
			if signal.Name == "SIGKILL" || signal.Name == "Terminate" {
				style = d.theme.Style(theme.Critical).Reverse(true)
			}

			lines = append(lines, style.Width(inner).Render(line))

			continue
		}

		lines = append(lines, clip.Render(dim.Render(line)))
	}

	lines = append(lines, "", clip.Render(" "+dim.Render(line(d.keys.Hints(keymap.Kill)))))

	return dialog.Frame(ctx, d.theme.Colour(theme.Critical), strings.Join(lines, "\n"))
}
