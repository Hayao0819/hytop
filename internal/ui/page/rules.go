package page

import (
	"strings"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func horizontalRule(t *theme.Theme) layout.Item {
	return layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
		return t.Style(theme.Border).Render(strings.Repeat("─", max(0, ctx.Width())))
	}))
}

func verticalRule(t *theme.Theme) layout.Item {
	return layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
		line := t.Style(theme.Border).Render("│")
		lines := make([]string, max(0, ctx.Height()))
		for i := range lines {
			lines[i] = line
		}

		return strings.Join(lines, "\n")
	}))
}
