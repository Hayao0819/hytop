package page

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"
	"github.com/Hayao0819/reactea/v2/modal"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/proctable"
	"github.com/Hayao0819/hytop/internal/ui/widget/summary"
)

// Processes displays and filters the process table.
type Processes struct {
	reactea.Wrapper

	view  *store.ViewState
	keys  *keymap.Map
	box   *layout.Box
	head  *summary.Widget
	theme *theme.Theme
	table *proctable.Widget
	bar   *filterBar
	err   string
	note  string

	log    *logPane
	rule   layout.Item
	shaped bool

	settling bool
}

func NewProcesses(env Env, follow func() LogFollower) *Processes {
	p := &Processes{
		view:  env.View,
		keys:  env.Keys,
		theme: env.Theme,
		table: proctable.New(env.Store, env.View, env.Theme, env.Keys, env.Columns),
		log:   newLogPane(env.Theme, follow),
	}

	p.bar = newFilterBar(env, "/", p.apply, p.source, func() { p.box.FocusFirst() })
	p.head = summary.New(env.Store, env.Theme, env.Caps)

	p.rule = layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
		return rule(env.Theme, ctx.Width())
	}))

	p.box = layout.Column()
	p.shaped = p.view.Logs()
	p.setItems(p.shaped)

	p.Wrapper = reactea.Wrap(p.box)

	return p
}

const (
	logMin = 4
	logMax = 12
)

func (p *Processes) reshape() {
	on := p.view.Logs()
	if on == p.shaped {
		return
	}

	p.shaped = on
	p.setItems(on)

	if !on {
		p.log.Close()
	}
}

func (p *Processes) setItems(withLog bool) {
	items := []layout.Item{
		layout.Fixed(summary.Rows, p.head),
		p.rule,
		layout.Grow(3, p.table).Focusable(),
	}

	if withLog {
		items = append(items, layout.Grow(1, reactea.Func(p.renderLog)).Bounds(logMin, logMax))
	}

	p.box.SetItems(append(items, layout.Fixed(1, p.bar).Focusable())...)
}

func (p *Processes) apply(src string) error {
	expr, err := filter.Compile(src)
	if err != nil {
		return err
	}

	p.view.PushFilter(src, expr)

	return nil
}

func (p *Processes) source() string {
	src, _ := p.view.Filter()

	return src
}

func (p *Processes) Hints() []Hint { return p.keys.Hints(p.scope()) }

func (p *Processes) scope() keymap.Scope {
	switch {
	case p.bar.Filtering():
		return keymap.Filtering
	case p.view.Tree():
		return keymap.ProcessesTree
	default:
		return keymap.Processes
	}
}

// Selected returns the selected process ID.
func (p *Processes) Selected() (int, bool) {
	proc, ok := p.table.Selected()
	if !ok {
		return 0, false
	}

	return proc.PID, true
}

func (p *Processes) renderLog(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{p.theme.Style(theme.Dim).Render(" journal   " + p.following())}
	lines = append(lines, p.log.Lines(width, height-1)...)

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (p *Processes) following() string {
	if unit := p.log.Unit(); unit != "" {
		return unit
	}

	return "this process belongs to no unit"
}

// settle debounces journal follower restarts during cursor movement.
const settle = 400 * time.Millisecond

type logSettled struct{}

// Update synchronizes the journal follower after the child table updates.
func (p *Processes) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if _, ok := msg.(logSettled); ok {
		p.settling = false

		if p.view.Logs() {
			p.log.Follow(ctx, p.selectedUnit())
		}

		return nil
	}

	cmd := p.update(ctx, msg)

	p.reshape()

	if !p.view.Logs() {
		return cmd
	}

	return tea.Batch(cmd, p.watch())
}

func (p *Processes) watch() tea.Cmd {
	if p.settling || p.selectedUnit() == p.log.Unit() {
		return nil
	}

	p.settling = true

	return tea.Tick(settle, func(time.Time) tea.Msg { return logSettled{} })
}

func (p *Processes) selectedUnit() string {
	proc, ok := p.table.Selected()
	if !ok {
		return ""
	}

	return proc.Unit
}

func (p *Processes) toggleLogs() tea.Cmd {
	p.view.ToggleLogs()

	return nil
}

func (p *Processes) update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if answer, ok := msg.(modal.Result[Choice]); ok {
		return p.signal(answer)
	}

	if ctx.InputCaptured() {
		return p.Wrapper.Update(ctx, msg)
	}

	scope := p.scope()

	switch {
	case p.keys.Is(msg, scope, keymap.ToggleLogs):
		return p.toggleLogs()

	case p.keys.Is(msg, scope, keymap.Signal):
		return p.ask(ctx)

	case p.keys.Is(msg, scope, keymap.Filter):
		p.err = ""
		p.box.FocusLast()

		return p.bar.Open(ctx)

	case p.keys.Is(msg, scope, keymap.FilterContainer):
		return p.filterContainer()

	case p.keys.Is(msg, scope, keymap.ToggleKernel):
		p.view.ToggleKernel()
		p.view.ResetSelection()

		return nil

	case p.keys.Is(msg, scope, keymap.ToggleTree):
		p.view.ToggleTree()
		p.view.ResetSelection()

		return nil

	case p.keys.Is(msg, scope, keymap.Fold):
		return p.fold()

	case p.keys.Is(msg, scope, keymap.Drill):
		return p.drill()

	case p.keys.Is(msg, scope, keymap.Back):
		return p.back()
	}

	return p.Wrapper.Update(ctx, msg)
}

// filterContainer uses the normal filter stack so Back restores the prior query.
func (p *Processes) filterContainer() tea.Cmd {
	proc, ok := p.table.Selected()
	if !ok || proc.Container == "" {
		p.note = "the selected process is not in a recognized container"

		return nil
	}

	src := fmt.Sprintf("container == %q", proc.Container)
	expr, err := filter.Compile(src)
	if err != nil {
		p.err = err.Error()

		return nil
	}

	p.view.PushFilter(src, expr)
	p.err = ""
	p.note = "showing " + proc.Container

	return nil
}

func (p *Processes) ask(ctx *reactea.Ctx) tea.Cmd {
	proc, ok := p.table.Selected()
	if !ok {
		return nil
	}

	command := proc.Cmdline
	if command == "" {
		command = proc.Name
	}

	p.err, p.note = "", ""

	return modal.PushAt(ctx, newKillDialog(p.theme, p.keys, proc.Identity(), command), modal.Centered(52, 13))
}

func (p *Processes) signal(answer modal.Result[Choice]) tea.Cmd {
	if !answer.Ok() {
		p.err = answer.Err.Error()

		return nil
	}

	if err := send(answer.Value); err != nil {
		p.err = err.Error()

		return nil
	}

	p.err = ""
	p.note = fmt.Sprintf("sent %s to %d", answer.Value.Name, answer.Value.Identity.PID)

	return nil
}

func (p *Processes) fold() tea.Cmd {
	if !p.view.Tree() {
		return nil
	}

	proc, ok := p.table.Selected()
	if !ok {
		return nil
	}

	p.view.ToggleCollapsed(proc.Identity())

	return nil
}

func (p *Processes) drill() tea.Cmd {
	pid, ok := p.Selected()
	if !ok {
		return nil
	}

	src := fmt.Sprintf("subtree(pid == %d)", pid)

	expr, err := filter.Compile(src)
	if err != nil {
		p.err = err.Error()

		return nil
	}

	p.view.PushFilter(src, expr)
	p.err = ""

	return nil
}

func (p *Processes) back() tea.Cmd {
	previous, ok := p.view.PopFilter()
	if !ok {
		return nil
	}

	expr, err := filter.Compile(previous)
	if err != nil {
		p.err = err.Error()

		return nil
	}

	p.view.SetFilter(previous, expr)
	p.view.ResetSelection()

	return nil
}

// Error returns the current page error.
func (p *Processes) Error() string {
	if p.err != "" {
		return p.err
	}
	if err := p.table.Error(); err != nil {
		return err.Error()
	}

	return p.bar.Error()
}

// Note returns the current page status message.
func (p *Processes) Note() string { return p.note }
