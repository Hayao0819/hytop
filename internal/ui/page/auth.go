package page

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type PasswordChoice struct{ Password string }

type passwordDialog struct {
	reactea.Wrapper

	theme  *theme.Theme
	keys   *keymap.Map
	reason string
	input  *reactea.ReactifiedWidget[textinput.Model]
}

func newPasswordDialog(t *theme.Theme, keys *keymap.Map, reason string) *passwordDialog {
	in := textinput.New()
	in.Prompt = " Password: "
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = '•'
	in.SetVirtualCursor(false)
	in.Focus()

	widget := reactea.ReactifyWidget(in).OnResize(func(m textinput.Model, width, _ int) textinput.Model {
		m.SetWidth(max(1, width-2))
		return m
	})

	d := &passwordDialog{theme: t, keys: keys, reason: reason, input: widget}
	d.Wrapper = reactea.Wrap(widget)

	return d
}

func (d *passwordDialog) Init(ctx *reactea.Ctx) tea.Cmd {
	return d.Wrapper.Init(ctx)
}

func (d *passwordDialog) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case d.keys.Is(msg, keymap.Auth, keymap.Cancel):
		d.input.Widget.SetValue("")
		return modal.Dismiss(ctx)
	case d.keys.Is(msg, keymap.Auth, keymap.Apply):
		password := d.input.Widget.Value()
		d.input.Widget.SetValue("")
		return modal.Return(ctx, PasswordChoice{Password: password})
	}

	return d.Wrapper.Update(ctx, msg)
}

func (d *passwordDialog) Render(ctx *reactea.Ctx) string {
	inner := dialog.InnerWidth(ctx)
	dim := d.theme.Style(theme.Dim)
	lines := []string{
		d.theme.Style(theme.Heading).Width(inner).Render(" Administrator authentication"),
		"",
		lipgloss.NewStyle().MaxWidth(inner).Render(" " + d.reason),
		"",
		d.Wrapper.Render(ctx),
		"",
		dim.Render(" The password is sent once to sudo through standard input."),
		dim.Render(" enter continue   esc cancel"),
	}

	return dialog.Frame(ctx, d.theme.Colour(theme.BorderActive), strings.Join(lines, "\n"))
}
