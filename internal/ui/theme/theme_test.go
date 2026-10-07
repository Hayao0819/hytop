package theme_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func TestMonoThemeDoesNotExposeANSIColours(t *testing.T) {
	t.Parallel()

	mono := theme.Build(render.Caps{Glyphs: render.ASCII, Colors: render.Mono}, nil)
	drawn := lipgloss.NewStyle().Foreground(mono.Colour(theme.CPU)).Render("plot")

	if strings.Contains(drawn, "\x1b[") {
		t.Fatalf("mono colour emitted an ANSI sequence: %q", drawn)
	}
}

func TestColourThemeStillExposesItsPalette(t *testing.T) {
	t.Parallel()

	coloured := theme.Build(render.Caps{Glyphs: render.Block, Colors: render.Ansi16}, nil)
	drawn := lipgloss.NewStyle().Foreground(coloured.Colour(theme.CPU)).Render("plot")

	if !strings.Contains(drawn, "\x1b[") {
		t.Fatalf("colour theme did not emit an ANSI sequence: %q", drawn)
	}
}

func TestSummaryLabelsNeverUseFaintText(t *testing.T) {
	t.Parallel()

	standard := theme.Build(render.Caps{Glyphs: render.Braille, Colors: render.Ansi256}, nil)
	if drawn := standard.Style(theme.Label).Render("Tasks"); strings.Contains(drawn, "\x1b[2m") {
		t.Fatalf("label is faint: %q", drawn)
	}

	high := theme.BuildWith(
		render.Caps{Glyphs: render.Braille, Colors: render.Ansi256},
		nil,
		theme.Options{HighContrast: true},
	)
	if drawn := high.Style(theme.Dim).Render("secondary"); strings.Contains(drawn, "\x1b[2m") {
		t.Fatalf("high-contrast secondary text is faint: %q", drawn)
	}
}

func TestAutomaticContrastUsesTheTerminalForeground(t *testing.T) {
	t.Parallel()

	caps := render.Caps{Glyphs: render.Braille, Colors: render.Ansi256}
	automatic := theme.BuildWith(caps, nil, theme.Options{})
	muted := theme.BuildWith(caps, nil, theme.Options{Muted: true})

	if drawn := automatic.Style(theme.Dim).Render("description"); strings.Contains(drawn, "\x1b[") {
		t.Fatalf("automatic contrast forced a dark grey: %q", drawn)
	}
	if drawn := muted.Style(theme.Dim).Render("description"); !strings.Contains(drawn, "\x1b[") {
		t.Fatalf("standard contrast did not use its muted colour: %q", drawn)
	}
}

func TestLightBackgroundUsesDarkerActiveColours(t *testing.T) {
	t.Parallel()

	dark, light := true, false
	caps := render.Caps{Glyphs: render.Block, Colors: render.Ansi16}
	onDark := theme.BuildWith(caps, nil, theme.Options{Dark: &dark})
	onLight := theme.BuildWith(caps, nil, theme.Options{Dark: &light})

	if onDark.Colour(theme.Heading) == onLight.Colour(theme.Heading) {
		t.Error("light and dark backgrounds resolved to the same heading colour")
	}
}
