package page

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"
	"github.com/Hayao0819/reactea/v2/router"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/summary"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

// Mounts supplies the current filesystem list.
type Mounts func() []diskmodel.Mount

// Drives supplies the current drive list.
type Drives func() []diskmodel.Device

type RefreshDrives func(context.Context, string) error

// Scanner is the directory-scanning interface used by the usage page.
type Scanner interface {
	Start(ctx context.Context, root string)
	Restart(ctx context.Context, root string)
	Stop()
	Scan() diskmodel.Scan
	Running() bool
	CanElevate() bool
	Remove(path string) error
	RemoveElevated(context.Context, string, string) error
	RestartElevated(context.Context, string, string) error
}

type diskPage struct {
	Title string
	Route string
}

type storageRail struct {
	reactea.BasicComponent

	storage *Storage
}

func (r *storageRail) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok || click.Button != tea.MouseLeft {
		return nil
	}

	_, y, inside := reactea.Mouse(ctx, msg)
	index := y / 2
	if !inside || index < 0 || index >= len(r.storage.rail) {
		return nil
	}

	return ctx.SetRoute(r.storage.rail[index].Route)
}

func (r *storageRail) Render(ctx *reactea.Ctx) string { return r.storage.renderRail(ctx) }

func DiskPages() []diskPage {
	return []diskPage{
		{"Filesystems", "/disk/filesystems"},
		{"Drives", "/disk/drives"},
		{"Usage", "/disk/usage"},
	}
}

// Storage is the filesystem, drive, and disk-usage mode.
type Storage struct {
	reactea.Wrapper

	keys  *keymap.Map
	theme *theme.Theme
	pages *router.Component
	rail  []diskPage
	box   *layout.Box
	head  *summary.Disk
	usage *usage
}

const (
	railItem = "rail"
	paneItem = "pane"
)

func (d *Storage) inside() bool { return d.box.FocusedKey() == paneItem }

func NewStorage(env Env, mounts Mounts, drives Drives, refresh RefreshDrives, scan func() Scanner) *Storage {
	d := &Storage{keys: env.Keys, theme: env.Theme, rail: DiskPages()}

	if mounts == nil {
		mounts = func() []diskmodel.Mount { return nil }
	}

	if drives == nil {
		drives = func() []diskmodel.Device { return nil }
	}

	usage := newUsage(env, scan)
	d.usage = usage
	drivesPage := newDrives(env, drives, refresh)

	filesystems := newFilesystems(env, mounts, func(ctx *reactea.Ctx, at string) tea.Cmd {
		usage.At(at)
		d.box.FocusKey(paneItem)

		return ctx.SetRoute("/disk/usage")
	})

	d.pages = router.NewWithRoutes(router.Routes{
		"/disk/filesystems": func(router.Params) reactea.Component { return filesystems },
		"/disk/drives":      func(router.Params) reactea.Component { return drivesPage },
		"/disk/usage":       func(router.Params) reactea.Component { return usage },
		"default":           func(router.Params) reactea.Component { return filesystems },
	})

	d.head = summary.NewDisk(env.Store, env.Theme, env.Caps, mounts, drives)
	rail := &storageRail{storage: d}

	d.box = layout.Row(
		layout.Fixed(14, rail).Key(railItem).Focusable(),
		layout.Spacer(1),
		layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
			return divider(env.Theme, ctx.Height())
		})),
		layout.Spacer(1),
		layout.Grow(4, d.pages).Key(paneItem).Focusable(),
	)

	d.box.FocusKey(railItem)

	d.Wrapper = reactea.Wrap(layout.Column(
		layout.Fixed(summary.Rows, d.head),
		layout.Fixed(1, reactea.Func(func(ctx *reactea.Ctx) string {
			return rule(env.Theme, ctx.Width())
		})),
		layout.Grow(1, d.box).Focusable(),
	))

	return d
}

// Hints returns bindings for the focused rail or page.
func (d *Storage) Hints() []Hint {
	if !d.inside() {
		return d.keys.Hints(keymap.Disk)
	}

	if hinter, ok := d.pages.Current().(Hinter); ok {
		if hints := hinter.Hints(); hints != nil {
			return hints
		}
	}

	return d.keys.Hints(keymap.Disk)
}

func (d *Storage) Note() string {
	return NoteOf(d.pages.Current())
}

func (d *Storage) Error() string {
	return ErrorOf(d.pages.Current())
}

func (d *Storage) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if ctx.InputCaptured() {
		return d.Wrapper.Update(ctx, msg)
	}

	switch {
	case d.keys.Is(msg, keymap.Disk, keymap.NextPane):
		return d.step(ctx, 1)

	case d.keys.Is(msg, keymap.Disk, keymap.PrevPane):
		return d.step(ctx, -1)

	case d.keys.Is(msg, keymap.Disk, keymap.EnterPane):
		d.box.FocusKey(paneItem)

		return nil

	case d.keys.Is(msg, keymap.Disk, keymap.LeavePane):
		d.box.FocusKey(railItem)

		return nil
	}

	if !d.inside() {
		switch {
		case d.keys.Is(msg, keymap.Disk, keymap.Down):
			return d.step(ctx, 1)

		case d.keys.Is(msg, keymap.Disk, keymap.Up):
			return d.step(ctx, -1)

		case d.keys.Is(msg, keymap.Disk, keymap.Open):
			d.box.FocusKey(paneItem)

			return nil
		}
	}

	return d.Wrapper.Update(ctx, msg)
}

func (d *Storage) step(ctx *reactea.Ctx, by int) tea.Cmd {
	current := 0

	for i, page := range d.rail {
		if page.Route == ctx.Route() {
			current = i
		}
	}

	return ctx.SetRoute(d.rail[(current+by+len(d.rail))%len(d.rail)].Route)
}

func (d *Storage) renderRail(ctx *reactea.Ctx) string {
	lines := make([]string, 0, ctx.Height())

	for _, page := range d.rail {
		style := d.theme.Style(theme.Text)
		marker := "  "

		if page.Route == ctx.Route() {
			style = d.theme.Style(theme.Heading)

			token := theme.BorderActive
			if d.inside() {
				token = theme.Border
			}

			marker = d.theme.Style(token).Render("▎") + " "
		}

		lines = append(lines, marker+style.Render(page.Title), "")
	}

	for len(lines) < ctx.Height() {
		lines = append(lines, "")
	}

	return strings.Join(lines[:max(0, ctx.Height())], "\n")
}

func bar(t *theme.Theme, fraction float64, width int) string {
	token := t.Threshold(fraction*100, 75, 90)
	if token == theme.Text || token == theme.Dim {
		token = theme.DiskWrite
	}

	filled, rest := chart.Gauge(fraction, width, uirender.Block)

	return t.Style(token).Render(filled) + t.Style(theme.Dim).Render(rest)
}

func plainBar(fraction float64, width int) string {
	filled, rest := chart.Gauge(fraction, width, uirender.Block)

	return filled + rest
}

func heading(t *theme.Theme, width int, text string) string {
	return t.Style(theme.TableHeader).Width(width).Render(text)
}

func fit(text string, width int) string {
	return uirender.Left(text, width)
}

func fitRight(text string, width int) string {
	return uirender.Right(text, width)
}

func percent(value float64) string { return fmt.Sprintf("%.0f %%", value) }
