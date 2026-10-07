package app

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

type setup struct {
	reactea.BasicComponent

	base     conf.Config
	config   conf.Config
	caps     render.Caps
	dark     *bool
	path     string
	save     func(conf.Config) error
	step     int
	problem  string
	complete bool
}

func newSetup(base conf.Config, caps render.Caps, path string, save func(conf.Config) error) *setup {
	return &setup{base: base, config: base.Clone(), caps: caps, path: path, save: save}
}

func (s *setup) Init(*reactea.Ctx) tea.Cmd {
	return tea.RequestBackgroundColor
}

func (s *setup) Update(_ *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		dark := msg.IsDark()
		s.dark = &dark

	case tea.KeyPressMsg:
		s.problem = ""

		switch msg.String() {
		case "ctrl+c":
			return tea.Quit
		case "up", "k", "left", "h":
			s.change(-1)
		case "down", "j", "right", "l", "space":
			s.change(1)
		case "r":
			s.config = s.base.Clone()
			s.step = setupReview
		case "esc", "backspace", "b":
			if s.step > setupContrast {
				s.step--
			}
		case "enter", "tab":
			if s.step < setupReview {
				s.step++

				return nil
			}

			if s.save == nil {
				s.problem = "configuration cannot be saved"

				return nil
			}
			if err := s.config.Validate(); err != nil {
				s.problem = err.Error()

				return nil
			}
			if err := s.save(s.config); err != nil {
				s.problem = err.Error()

				return nil
			}

			s.complete = true

			return tea.Quit
		}
	}

	return nil
}

func (s *setup) change(by int) {
	item, ok := s.item()
	if !ok {
		return
	}

	current := index(item.Options, item.Value(s.config))
	next := (current + by + len(item.Options)) % len(item.Options)
	item.Apply(&s.config, item.Options[next])
}

func (s *setup) item() (preference, bool) {
	for _, item := range preferences() {
		if item.Step == s.step {
			return item, true
		}
	}

	return preference{}, false
}

func (s *setup) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	if width < 28 || height < 7 {
		message := strings.Join([]string{
			"hytop setup",
			"",
			fmt.Sprintf("terminal is %d×%d", width, height),
			"resize to at least 28×7",
			"",
			"ctrl+c exit",
		}, "\n")

		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, message)
	}

	inner := min(width-4, 76)
	t := s.currentTheme()
	compact := height < 16
	header := render.Sides(
		t.Style(theme.Heading).Bold(true).Render("hytop setup"),
		t.Style(theme.Dim).Render(fmt.Sprintf("%d / %d", s.step+1, setupReview+1)),
		inner,
	)
	lines := []string{header}
	if !compact {
		lines = append(lines, "")
	}

	if s.step == setupReview {
		lines = append(lines, s.review(t, compact)...)
	} else {
		item, _ := s.item()
		lines = append(lines, t.Style(theme.Heading).Render(item.Label))
		if !compact {
			lines = append(lines, t.Style(theme.Dim).Render(item.What), "")
		}
		lines = append(lines, s.choices(t)...)
		if !compact && (s.step == setupContrast || s.step == setupGlyphs) {
			lines = append(lines, "")
			lines = append(lines, s.preview(t, inner)...)
		}
	}

	if s.problem != "" {
		if !compact {
			lines = append(lines, "")
		}
		lines = append(lines, t.Style(theme.Critical).Render("! "+safe.Text(s.problem)))
	}

	footer := "↑/↓ choose   enter continue   esc back   r defaults   ctrl+c exit"
	if s.step == setupReview {
		footer = "enter save and start   esc back   r defaults   ctrl+c exit"
	}
	if inner < 58 {
		footer = "↑/↓ choose   enter continue   ctrl+c exit"
		if s.step == setupReview {
			footer = "enter save   esc back   ctrl+c exit"
		}
	}
	if !compact {
		lines = append(lines, "")
	}
	lines = append(lines, t.Style(theme.Dim).Render(footer))

	for i, line := range lines {
		lines[i] = render.Ellipsize(line, inner)
	}
	if len(lines) > height {
		lines = append(lines[:height-1], lines[len(lines)-1])
	}

	content := strings.Join(lines, "\n")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (s *setup) currentTheme() *theme.Theme {
	caps := s.config.Caps(s.caps)

	return theme.BuildWith(caps, s.config.Palette(), s.config.ThemeOptions(s.dark))
}

func (s *setup) choices(t *theme.Theme) []string {
	item, ok := s.item()
	if !ok {
		return nil
	}

	current := item.Value(s.config)
	lines := make([]string, 0, len(item.Options))
	for _, option := range item.Options {
		marker := t.Style(theme.Dim).Render("  ○ ")
		value := t.Style(theme.Label).Render(option)
		if option == current {
			marker = t.Style(theme.BorderActive).Render("  ● ")
			value = t.Style(theme.Selected).Render(" " + option + " ")
		}
		lines = append(lines, marker+value)
	}

	return lines
}

func (s *setup) preview(t *theme.Theme, width int) []string {
	label := t.Style(theme.Label)
	lines := []string{t.Style(theme.Heading).Render("Preview")}
	if s.step == setupContrast {
		return append(lines,
			label.Render("Tasks ")+t.Style(theme.Text).Bold(true).Render("184")+
				label.Render("  running ")+t.Style(theme.Own).Bold(true).Render("3")+
				label.Render("  sleeping ")+t.Style(theme.Text).Render("178"),
			label.Render("Disk  read ")+t.Style(theme.DiskRead).Render("18.4 MiB/s")+
				label.Render("  write ")+t.Style(theme.DiskWrite).Render("3.2 MiB/s"),
		)
	}

	graphWidth := min(max(width-6, 12), 48)
	values := []float64{18, 24, 30, 62, 48, 72, 66, 80, 54, 42, 58, 76}
	grid := chart.Line{
		Series: [][]float64{values}, Glyphs: s.config.Caps(s.caps).Glyphs, Fill: true,
	}.Render(graphWidth, 2)
	lines = append(lines, grid.Lines(func(owner chart.SeriesID) lipgloss.Style {
		if owner == chart.Empty {
			return lipgloss.NewStyle()
		}

		return t.Style(theme.CPU)
	})...)

	return lines
}

func (s *setup) review(t *theme.Theme, compact bool) []string {
	value := func(label, value string) string {
		return t.Style(theme.Label).Render(fmt.Sprintf("%-20s", label)) +
			t.Style(theme.Text).Bold(true).Render(value)
	}
	if compact {
		return []string{
			value("Text contrast", s.config.Appearance.Contrast),
			value("Graph style", s.config.Graphs.Glyphs),
			value("Collection period", s.config.General.Interval.String()),
			value("Process tree", boolName(s.config.Processes.Tree)),
			value("Write", safe.Text(s.path)),
		}
	}

	return []string{
		t.Style(theme.Heading).Render("Ready to start"),
		value("Text contrast", s.config.Appearance.Contrast),
		value("Graph style", s.config.Graphs.Glyphs),
		value("Collection period", s.config.General.Interval.String()),
		value("Process tree", boolName(s.config.Processes.Tree)),
		"",
		t.Style(theme.Dim).Render("The initial settings will be written to"),
		t.Style(theme.Label).Render(safe.Text(s.path)),
	}
}

// RunSetup displays the first-run setup and reports whether it was saved.
func RunSetup(
	ctx context.Context,
	base conf.Config,
	caps render.Caps,
	path string,
	save func(conf.Config) error,
) (bool, error) {
	wizard := newSetup(base, caps, path, save)
	program := reactea.New(wizard,
		reactea.WithAltScreen(),
		reactea.WithWindowTitle("hytop setup"),
	)

	if err := program.Run(tea.WithContext(ctx)); err != nil {
		return false, errors.Wrap(err, "running initial setup")
	}

	return wizard.complete, nil
}
