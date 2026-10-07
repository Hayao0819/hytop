package page

import (
	"strings"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

var unitColumns = []struct {
	title string
	width int
}{
	{"UNIT", 38},
	{"LOAD", 8},
	{"ACTIVE", 9},
	{"SUB", 10},
	{"DESCRIPTION", 0},
}

func header(width int) string {
	var b strings.Builder

	for _, column := range unitColumns {
		b.WriteString(pad(column.title, column.width, width-b.Len()))
		b.WriteByte(' ')
	}

	return b.String()
}

func unitLine(unit unitmodel.Unit, width int) string {
	var (
		b strings.Builder
		// Sanitize external unit metadata before applying styles.
		values = []string{
			safe.Text(unit.Name), unit.Load, unit.Active, unit.Sub, safe.Text(unit.Description),
		}
	)

	for i, column := range unitColumns {
		b.WriteString(pad(values[i], column.width, width-b.Len()))
		b.WriteByte(' ')
	}

	return b.String()
}

func field(t *theme.Theme, label, value string, width int) string {
	return t.Style(theme.Dim).Render(" "+pad(label, 14, width)) + uirender.Clip(value, width-15)
}

func pad(value string, width, remaining int) string {
	if width == 0 {
		width = max(0, remaining-1)
	}

	return uirender.Left(value, width)
}
