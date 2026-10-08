package page

import (
	"context"
	"sort"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/summary"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

// Services shows the unit list and the selected unit's state and log.
type Services struct {
	reactea.Wrapper

	theme *theme.Theme
	keys  *keymap.Map
	view  *store.ViewState
	units func() []unitmodel.Unit

	box  *layout.Box
	bar  *filterBar
	head *summary.Services

	cursor selection.Cursor
	opened string
	log    *logPane

	// query is rebuilt only when built changes.
	query unitmodel.Query
	built string
}

// LogFollower provides the journal operations used by pages.
type LogFollower interface {
	Follow(ctx context.Context, unit string)
	Entries() []unitmodel.LogEntry
	Err() error
}

func NewServices(env Env, list func() []unitmodel.Unit, follow func() LogFollower) *Services {
	s := &Services{
		theme: env.Theme,
		keys:  env.Keys,
		view:  env.View,
		units: list,
		log:   newLogPane(env.Theme, follow),
	}

	s.bar = newFilterBar(env, "/", s.search, env.View.ServiceSearch, func() { s.box.FocusFirst() })
	s.head = summary.NewServices(env.Store, env.Theme, env.Caps)

	s.box = layout.Column(
		layout.Fixed(summary.Rows, s.head),
		horizontalRule(env.Theme),
		layout.Grow(1, reactea.Func(s.render)).Focusable(),
		layout.Fixed(1, s.bar).Focusable(),
	)

	s.Wrapper = reactea.Wrap(s.box)

	return s
}

func (s *Services) search(text string) error {
	if _, err := unitmodel.Compile(text); err != nil {
		return err
	}

	s.view.SetServiceSearch(text)
	s.cursor.Reset()

	return nil
}

func (s *Services) Hints() []Hint { return s.keys.Hints(s.scope()) }

func (s *Services) Error() string { return s.bar.Error() }

func (s *Services) scope() keymap.Scope {
	switch {
	case s.bar.Filtering():
		return keymap.Filtering
	case s.opened != "":
		return keymap.Unit
	default:
		return keymap.Services
	}
}

// rows places failed units before the remaining alphabetical list.
func (s *Services) rows() []unitmodel.Unit {
	kept := make([]unitmodel.Unit, 0, 64)

	query := s.compiled()

	for _, unit := range s.units() {
		if !query.Matches(unit) {
			continue
		}

		kept = append(kept, unit)
	}

	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Failed() != kept[j].Failed() {
			return kept[i].Failed()
		}

		return kept[i].Name < kept[j].Name
	})

	return kept
}

func (s *Services) compiled() unitmodel.Query {
	text := s.view.ServiceSearch()
	if text == s.built {
		return s.query
	}

	query, err := unitmodel.Compile(text)
	if err != nil {
		query = unitmodel.Query{}
	}

	s.query, s.built = query, text

	return s.query
}

func (s *Services) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if ctx.InputCaptured() {
		return s.Wrapper.Update(ctx, msg)
	}

	scope := s.scope()

	switch {
	case s.keys.Is(msg, scope, keymap.Filter):
		s.box.FocusLast()

		return s.bar.Open(ctx)
	case s.keys.Is(msg, scope, keymap.Back):
		if s.opened != "" {
			s.close()

			return nil
		}

		s.bar.Clear()

	case s.keys.Is(msg, scope, keymap.Open):
		s.open(ctx)

	case s.keys.Is(msg, scope, keymap.Down):
		s.move(ctx, 1)

	case s.keys.Is(msg, scope, keymap.Up):
		s.move(ctx, -1)

	case s.keys.Is(msg, scope, keymap.Top):
		s.cursor.Top()

	case s.keys.Is(msg, scope, keymap.Bottom):
		s.cursor.Bottom(len(s.rows()))
		s.scroll(ctx)
	}

	return nil
}

// fields must match the detail rows reserved before the journal.
const fields = 6

func (s *Services) move(ctx *reactea.Ctx, by int) {
	if s.opened != "" {
		s.log.Scroll(-by)

		return
	}

	rows := len(s.rows())
	if rows == 0 {
		return
	}

	s.cursor.Move(by, rows)
	s.scroll(ctx)
}

func (s *Services) scroll(ctx *reactea.Ctx) {
	_, height := ctx.Size()

	visible := max(1, height-1)

	s.cursor.Reveal(visible, len(s.rows()))
}

func (s *Services) open(ctx *reactea.Ctx) {
	rows := s.rows()
	if s.cursor.Selected >= len(rows) {
		return
	}

	s.opened = rows[s.cursor.Selected].Name
	s.log.Follow(ctx, s.opened)
}

func (s *Services) close() {
	s.log.Close()

	s.opened = ""
}

func (s *Services) render(ctx *reactea.Ctx) string {
	if s.opened != "" {
		return s.detail(ctx)
	}

	return s.list(ctx)
}

func (s *Services) list(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	var (
		rows      = s.rows()
		emptyNote string
	)

	if search := s.view.ServiceSearch(); len(rows) == 0 && search != "" {
		emptyNote = s.theme.Style(theme.Dim).Render(" nothing matches " + search)
	}

	return render.Table(height,
		s.theme.Style(theme.TableHeader).Width(width).Render(header(width)),
		emptyNote, len(rows), s.cursor.Offset, func(index int) string {
			line := unitLine(rows[index], width)

			if index == s.cursor.Selected && ctx.Focused() {
				return s.theme.Style(theme.Selected).Width(width).Render(line)
			}

			return s.paint(rows[index], line)
		})
}

func (s *Services) paint(unit unitmodel.Unit, line string) string {
	switch {
	case unit.Failed():
		return s.theme.Style(theme.Critical).Render(line)
	case unit.Active == "active":
		return s.theme.Style(theme.Own).Render(line)
	default:
		return s.theme.Style(theme.Dim).Render(line)
	}
}
