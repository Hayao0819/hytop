package page

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

// Setting describes one adjustable value and its choices.
type Setting struct {
	Label   string
	What    string
	Options []string
	Chosen  int
	Apply   func(string)

	// Group names the section containing this setting.
	Group string
}

// Settings edits the active configuration without saving automatically.
type Settings struct {
	reactea.BasicComponent

	theme    *theme.Theme
	keys     *keymap.Map
	items    []*Setting
	selected int
	offset   int

	// A nil Save disables the write action.
	Save func() error
	Path string

	note string
}

func NewSettings(env Env, items ...*Setting) *Settings {
	return &Settings{theme: env.Theme, keys: env.Keys, items: items}
}

// Hints omits the write action when Save is nil.
func (s *Settings) Hints() []Hint {
	hints := s.keys.Hints(keymap.Settings)
	if s.Save != nil {
		return hints
	}

	write := s.keys.Keys(keymap.Settings, keymap.Write)

	kept := make([]Hint, 0, len(hints))

	for _, hint := range hints {
		if len(write) == 0 || hint.Key != write[0] {
			kept = append(kept, hint)
		}
	}

	return kept
}

func (s *Settings) Note() string { return s.note }

func (s *Settings) Update(_ *reactea.Ctx, msg tea.Msg) tea.Cmd {
	switch {
	case s.keys.Is(msg, keymap.Settings, keymap.Down):
		s.selected = min(s.selected+1, max(0, len(s.items)-1))

	case s.keys.Is(msg, keymap.Settings, keymap.Up):
		s.selected = max(s.selected-1, 0)

	case s.keys.Is(msg, keymap.Settings, keymap.More):
		s.step(1)

	case s.keys.Is(msg, keymap.Settings, keymap.Less):
		s.step(-1)

	case s.keys.Is(msg, keymap.Settings, keymap.Write):
		s.write()
	}

	return nil
}

func (s *Settings) write() {
	if s.Save == nil {
		return
	}

	if err := s.Save(); err != nil {
		s.note = err.Error()

		return
	}

	s.note = "written to " + s.Path
}

// Selected and Select carry the cursor across a rebuild, since changing a
// setting throws the page away and makes a new one.
func (s *Settings) Selected() int { return s.selected }

func (s *Settings) Select(row int) {
	s.selected = min(max(row, 0), max(0, len(s.items)-1))
}

func (s *Settings) step(by int) {
	if s.selected >= len(s.items) {
		return
	}

	item := s.items[s.selected]
	if len(item.Options) == 0 {
		return
	}

	item.Chosen = (item.Chosen + by + len(item.Options)) % len(item.Options)

	if item.Apply != nil {
		item.Apply(item.Options[item.Chosen])
	}
}

func (s *Settings) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	var lines []string
	group := ""
	selectedLine := 0

	for i, item := range s.items {
		if item.Group != group {
			group = item.Group

			if len(lines) > 0 {
				lines = append(lines, "")
			}

			lines = append(lines, s.theme.Style(theme.Heading).Render(" "+group))
		}

		if i == s.selected {
			selectedLine = len(lines)
		}
		lines = append(lines, s.row(width, item, i == s.selected && ctx.Focused()))
	}

	footerHeight := min(height, 2)
	bodyHeight := max(0, height-footerHeight)
	if selectedLine < s.offset {
		s.offset = selectedLine
	} else if selectedLine >= s.offset+bodyHeight {
		s.offset = selectedLine - bodyHeight + 1
	}
	start, end := selection.Window(s.offset, bodyHeight, len(lines))
	s.offset = start
	body := slices.Clone(lines[start:end])
	for len(body) < bodyHeight {
		body = append(body, "")
	}

	position := ""
	if len(lines) > bodyHeight {
		position = "  " + strings.Repeat("↑", btoi(s.offset > 0)) +
			strings.Repeat("↓", btoi(s.offset+bodyHeight < len(lines)))
	}
	footer := make([]string, 0, footerHeight)
	if footerHeight == 2 {
		description := ""
		if s.selected < len(s.items) {
			description = safe.Text(s.items[s.selected].What)
		}
		footer = append(footer, s.theme.Style(theme.Label).Render(" "+render.Ellipsize(description, max(0, width-1))))
	}
	footer = append(footer, s.theme.Style(theme.Dim).Render(" "+s.footer()+position))

	return strings.Join(append(body, footer...), "\n")
}

func btoi(ok bool) int {
	if ok {
		return 1
	}

	return 0
}

func (s *Settings) row(width int, item *Setting, selected bool) string {
	value := "—"

	if len(item.Options) > 0 {
		value = "‹ " + item.Options[item.Chosen] + " ›"
	}

	marker := "  "
	if selected {
		marker = s.theme.Style(theme.BorderActive).Render("▎") + " "
	}

	label := s.theme.Style(theme.Label).Render(safe.Text(item.Label))
	shown := s.theme.Style(theme.Dim).Render(value)

	if selected {
		label = s.theme.Style(theme.Label).Bold(true).Render(safe.Text(item.Label))
		shown = s.theme.Style(theme.Selected).Render(" " + value + " ")
	}

	return marker + render.Sides(label, shown, max(0, width-2))
}

func (s *Settings) footer() string {
	if s.Save == nil || s.Path == "" {
		return "changes apply now and are lost on exit"
	}

	return "changes apply now; w writes them to " + safe.Text(s.Path)
}
