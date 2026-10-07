package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/ui/dialog"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/version"
)

const wordmark = ` _           _
| |__  _   _| |_ ___  _ __
| '_ \| | | | __/ _ \| '_ \
| | | | |_| | || (_) | |_) |
|_| |_|\__, |\__\___/| .__/
       |___/          |_|`

type about struct {
	reactea.BasicComponent

	theme  *theme.Theme
	keys   *keymap.Map
	info   version.Info
	offset int
}

func newAbout(t *theme.Theme, keys *keymap.Map, info version.Info) *about {
	return &about{theme: t, keys: keys, info: info}
}

func aboutPlacement(ctx *reactea.Ctx) modal.Placement {
	width, height := ctx.Size()

	return modal.Centered(modalExtent(width, 68), modalExtent(height, 21))
}

func modalExtent(available, preferred int) int {
	if available > 2 {
		available -= 2
	}

	return min(available, preferred)
}

func (a *about) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case a.keys.Is(msg, keymap.AboutScreen, keymap.Cancel):
		return modal.Dismiss(ctx)

	case a.keys.Is(msg, keymap.AboutScreen, keymap.Down):
		a.offset++

	case a.keys.Is(msg, keymap.AboutScreen, keymap.Up):
		a.offset = max(0, a.offset-1)
	}

	return nil
}

func (a *about) Render(ctx *reactea.Ctx) string {
	var (
		inner   = dialog.InnerWidth(ctx)
		clip    = lipgloss.NewStyle().MaxWidth(inner)
		heading = a.theme.Style(theme.Heading)
		dim     = a.theme.Style(theme.Dim)
		commit  = a.info.Commit
	)

	if len(commit) > 12 {
		commit = commit[:12]
	}
	if a.info.Modified {
		commit += " (modified)"
	}

	row := func(label, value string) string {
		return clip.Render(" " + dim.Render(fmt.Sprintf("%-11s", label)) + safe.Text(value))
	}

	body := []string{""}
	for line := range strings.SplitSeq(wordmark, "\n") {
		body = append(body, heading.Width(inner).Align(lipgloss.Center).Render(line))
	}

	body = append(body,
		"",
		row("Version", a.info.Version),
		row("Commit", commit),
		row("Date", a.info.Date),
		row("Runtime", a.info.GoVersion+"  "+a.info.Platform),
		"",
		row("Repository", version.Repository),
		row("Developer", version.Developer),
		row("Twitter", version.Twitter),
		"",
	)

	bodyHeight := max(0, ctx.Height()-4)
	visible := scroll(body, &a.offset, bodyHeight)
	footer := " esc/q/i close "

	if len(body) > bodyHeight {
		footer = fmt.Sprintf(" %d–%d / %d   j/k scroll   esc close ",
			min(a.offset+1, len(body)), min(a.offset+len(visible), len(body)), len(body))
	}

	lines := append(
		[]string{heading.Width(inner).Align(lipgloss.Center).Render("about hytop · " + safe.Text(a.info.Version))},
		visible...,
	)
	lines = append(lines, dim.Width(inner).Align(lipgloss.Right).Render(footer))

	return dialog.Frame(ctx, a.theme.Colour(theme.BorderActive), strings.Join(lines, "\n"))
}
