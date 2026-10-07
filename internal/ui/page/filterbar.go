package page

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type filterBar struct {
	reactea.Wrapper
	reactea.InputCapture

	theme   *theme.Theme
	keys    *keymap.Map
	input   *reactea.ReactifiedWidget[textinput.Model]
	onClose func()

	commit  func(string) error
	current func() string

	open_ bool
	err   string
}

func newFilterBar(
	env Env, prompt string, commit func(string) error, current func() string, onClose func(),
) *filterBar {
	in := textinput.New()
	in.Prompt = prompt
	in.SetVirtualCursor(false)

	widget := reactea.ReactifyWidget(in).OnResize(func(m textinput.Model, w, _ int) textinput.Model {
		m.SetWidth(w)

		return m
	})

	return &filterBar{
		Wrapper: reactea.Wrap(widget),
		theme:   env.Theme,
		keys:    env.Keys,
		input:   widget,
		onClose: onClose,
		commit:  commit,
		current: current,
	}
}

// Open begins editing the active filter.
func (f *filterBar) Open(ctx *reactea.Ctx) tea.Cmd {
	f.open_, f.err = true, ""
	f.input.Widget.SetValue(f.current())
	f.input.Widget.CursorEnd()
	f.input.Widget.Focus()

	return f.CaptureInput(ctx)
}

// Filtering reports whether the bar owns keyboard input.
func (f *filterBar) Filtering() bool { return f.open_ }

// Error returns the last rejected filter error.
func (f *filterBar) Error() string { return f.err }

func (f *filterBar) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if !f.open_ {
		return nil
	}

	switch {
	case f.keys.Is(msg, keymap.Filtering, keymap.Cancel):
		return f.close()

	case f.keys.Is(msg, keymap.Filtering, keymap.Apply):
		if err := f.commit(f.input.Widget.Value()); err != nil {
			f.err = err.Error()

			return nil
		}

		return f.close()
	}

	f.err = ""

	return f.Wrapper.Update(ctx, msg)
}

// Clear removes the active filter without opening the editor.
func (f *filterBar) Clear() bool {
	if f.current() == "" {
		return false
	}

	f.err = ""
	_ = f.commit("")

	return true
}

func (f *filterBar) close() tea.Cmd {
	f.open_ = false
	f.input.Widget.Blur()

	if f.onClose != nil {
		f.onClose()
	}

	return f.ReleaseInput()
}

func (f *filterBar) Render(ctx *reactea.Ctx) string {
	if f.open_ {
		return f.Wrapper.Render(ctx)
	}

	src := f.current()
	if src == "" {
		return ""
	}

	prompt := f.input.Widget.Prompt
	if strings.HasPrefix(src, prompt) {
		// Avoid duplicating a delimiter already present in the expression.
		prompt = ""
	}

	return f.theme.Style(theme.Dim).Width(ctx.Width()).Render(prompt + src)
}
