package page

import (
	"strings"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// One systemd unit: what it is doing and what it has been saying.

func (s *Services) detail(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	var (
		unit  = s.current()
		lines = []string{s.theme.Style(theme.Heading).Width(width).Render(" " + s.opened)}
	)

	lines = append(lines,
		field(s.theme, "Description", unit.Description, width),
		field(s.theme, "Loaded", unit.Load, width),
		field(s.theme, "Active", unit.Active+" ("+unit.Sub+")", width),
		"",
		s.theme.Style(theme.Dim).Render(" journal"),
	)

	lines = append(lines, s.log.Lines(width, max(0, height-fields))...)

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (s *Services) current() unitmodel.Unit {
	for _, unit := range s.units() {
		if unit.Name == s.opened {
			return unit
		}
	}

	return unitmodel.Unit{Name: s.opened}
}
