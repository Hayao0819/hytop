package render

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	reactearender "github.com/Hayao0819/reactea/v2/render"
)

var (
	Clip      = reactearender.Clip
	Ellipsize = reactearender.Ellipsize
	Left      = reactearender.Left
	Right     = reactearender.Right
	Sides     = reactearender.Sides
)

// Table renders a header and row viewport padded to height lines.
func Table(height int, header, empty string, count, offset int, row func(int) string) string {
	if height <= 0 {
		return ""
	}
	count = max(0, count)
	offset = max(0, offset)

	lines := make([]string, 0, height)
	lines = append(lines, header)

	if height > 1 && count == 0 && empty != "" {
		lines = append(lines, empty)
	} else {
		for slot := 0; slot < height-1; slot++ {
			index := offset + slot
			if index >= count {
				break
			}

			lines = append(lines, row(index))
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func Foreground(caps Caps, colour color.Color) lipgloss.Style {
	style := lipgloss.NewStyle()
	if caps.Colors != Mono && colour != nil {
		style = style.Foreground(colour)
	}

	return style
}
