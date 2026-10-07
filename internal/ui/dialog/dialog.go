// Package dialog renders hytop modal frames.
package dialog

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
)

// InnerWidth is the content width inside the standard one-cell border.
func InnerWidth(ctx *reactea.Ctx) int { return max(1, ctx.Width()-2) }

// Frame fits content into the modal box and draws the standard rounded border.
// A nil colour leaves the terminal's foreground unchanged.
func Frame(ctx *reactea.Ctx, border color.Color, content string) string {
	// Omitting explicit sides avoids charmbracelet/lipgloss#732.
	style := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	if border != nil {
		style = style.BorderForeground(border)
	}

	return style.
		Width(ctx.Width()).Height(ctx.Height()).
		MaxWidth(ctx.Width()).MaxHeight(ctx.Height()).
		Render(content)
}
