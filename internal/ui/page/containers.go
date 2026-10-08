package page

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/proctable"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

const (
	containerListItem = "containers"
	containerProcItem = "processes"
)

// Containers derives its inventory and filtered table from one process snapshot.
type Containers struct {
	reactea.Wrapper

	store  *store.Store
	theme  *theme.Theme
	keys   *keymap.Map
	box    *layout.Box
	cursor selection.Cursor
	view   *store.ViewState
	table  *proctable.Widget
	key    string
	set    bool
}

func NewContainers(env Env) *Containers {
	view := store.NewViewState()
	view.SetKernel(true)

	columns, _ := procmodel.ResolveColumns([]string{"pid", "user", "cpu", "rss", "state", "command"})
	c := &Containers{
		store: env.Store,
		theme: env.Theme,
		keys:  env.Keys,
		view:  view,
	}
	c.table = proctable.New(env.Store, view, env.Theme, env.Keys, columns).WithScope(keymap.ContainerProcesses)
	c.selectContainer("")

	c.box = layout.Row(
		layout.Grow(24, reactea.Func(c.renderList)).Bounds(34, 42).Key(containerListItem).Focusable(),
		layout.Spacer(1),
		verticalRule(env.Theme),
		layout.Spacer(1),
		layout.Grow(1, c.table).Key(containerProcItem).Focusable(),
	)
	c.box.FocusKey(containerListItem)

	c.Wrapper = reactea.Wrap(layout.Column(
		layout.Fixed(1, reactea.Func(c.renderTitle)),
		horizontalRule(env.Theme),
		layout.Grow(1, c.box).Focusable(),
	))

	return c
}

func (c *Containers) inside() bool { return c.box.FocusedKey() == containerProcItem }

func (c *Containers) Hints() []Hint {
	if c.inside() {
		return c.keys.Hints(keymap.ContainerProcesses)
	}

	return c.keys.Hints(keymap.Containers)
}

func (c *Containers) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	c.sync(ctx.Height())

	if ctx.InputCaptured() {
		return c.Wrapper.Update(ctx, msg)
	}

	if c.keys.Is(msg, keymap.ContainerProcesses, keymap.LeavePane) {
		c.box.FocusKey(containerListItem)

		return nil
	}

	if !c.inside() {
		containers := c.containers()

		if c.keys.Is(msg, keymap.Containers, keymap.EnterPane) {
			if len(containers) > 0 {
				c.box.FocusKey(containerProcItem)
			}

			return nil
		}
		if !moveCursor(msg, c.keys, keymap.Containers, &c.cursor, len(containers)) {
			return c.Wrapper.Update(ctx, msg)
		}

		c.cursor.Reveal(max(1, ctx.Height()-3), len(containers))
		c.use(containers)

		return nil
	}

	return c.Wrapper.Update(ctx, msg)
}

func (c *Containers) containers() []procmodel.Container {
	return c.store.Processes().Containers()
}

func (c *Containers) sync(height int) {
	containers := c.containers()
	for index := range containers {
		if containers[index].Key == c.key {
			c.cursor.Selected = index
			break
		}
	}
	c.cursor.Clamp(len(containers))
	c.cursor.Reveal(max(1, height-3), len(containers))
	c.use(containers)
}

func (c *Containers) use(containers []procmodel.Container) {
	if len(containers) == 0 {
		c.selectContainer("")

		return
	}

	c.selectContainer(containers[c.cursor.Selected].Key)
}

func (c *Containers) selectContainer(key string) {
	if c.set && key == c.key {
		return
	}

	c.key = key
	c.set = true
	src := `container == "__no_container__"`
	if key != "" {
		src = fmt.Sprintf("container == %q", key)
	}

	expr, _ := filter.Compile(src)
	c.view.SetFilter(src, expr)
	c.view.ResetSelection()
}

func (c *Containers) renderTitle(ctx *reactea.Ctx) string {
	containers := c.containers()
	processes := 0
	for _, container := range containers {
		processes += container.Totals.Procs
	}

	title := fmt.Sprintf("Containers  %d containers  %d processes", len(containers), processes)
	return c.theme.Style(theme.Heading).Width(ctx.Width()).Render(title)
}

func (c *Containers) renderList(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	containers := c.containers()
	c.cursor.Clamp(len(containers))

	empty := ""
	if len(containers) == 0 {
		empty = c.theme.Style(theme.Dim).Render(" no container cgroups found")
	}

	header := c.theme.Style(theme.TableHeader).Width(width).Render(containerHeader(width))

	return render.Table(height, header, empty, len(containers), c.cursor.Offset, func(index int) string {
		line := containerLine(containers[index], width)
		if index == c.cursor.Selected {
			token := theme.Own
			if !c.inside() && ctx.Focused() {
				token = theme.Selected
			}

			return c.theme.Style(token).Width(width).Render(line)
		}

		return c.theme.Style(theme.Text).Render(line)
	})
}

func containerHeader(width int) string {
	return fitContainerLine("CONTAINER", "N", "CPU", "MEM", width)
}

func containerLine(container procmodel.Container, width int) string {
	return fitContainerLine(
		safe.Text(container.Key),
		fmt.Sprint(container.Totals.Procs),
		fmt.Sprintf("%.1f", container.Totals.CPU),
		series.Bytes.Format(float64(container.Totals.RSS), 0, series.Auto),
		width,
	)
}

func fitContainerLine(name, procs, cpu, memory string, width int) string {
	const fixed = 4 + 7 + 9
	nameWidth := max(8, width-fixed)
	name = lipgloss.NewStyle().MaxWidth(nameWidth).Render(strings.TrimSpace(name))

	return fmt.Sprintf("%-*s %3s %6s %8s", nameWidth, name, procs, cpu, memory)
}
