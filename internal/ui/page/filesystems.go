package page

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	uirender "github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

type filesystems struct {
	reactea.BasicComponent

	theme *theme.Theme
	keys  *keymap.Map
	list  Mounts

	open func(*reactea.Ctx, string) tea.Cmd

	cursor selection.Cursor

	// top is the first clickable row from the last render.
	top int
}

func newFilesystems(env Env, mounts Mounts, open func(*reactea.Ctx, string) tea.Cmd) *filesystems {
	return &filesystems{theme: env.Theme, keys: env.Keys, list: mounts, open: open}
}

func (f *filesystems) Hints() []Hint { return f.keys.Hints(keymap.Disk) }

func (f *filesystems) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	mounts := f.list()
	rows := len(mounts)

	if cmd, hit := f.clicked(ctx, msg, mounts); hit {
		return cmd
	}

	switch {
	case f.keys.Is(msg, keymap.Disk, keymap.Open):
		if f.cursor.Selected < rows && f.open != nil {
			return f.open(ctx, mounts[f.cursor.Selected].Path)
		}

	case f.keys.Is(msg, keymap.Disk, keymap.Down):
		f.cursor.Move(1, rows)
	case f.keys.Is(msg, keymap.Disk, keymap.Up):
		f.cursor.Move(-1, rows)
	case f.keys.Is(msg, keymap.Disk, keymap.Top):
		f.cursor.Top()
	case f.keys.Is(msg, keymap.Disk, keymap.Bottom):
		f.cursor.Bottom(rows)
	}

	return nil
}

func (f *filesystems) clicked(
	ctx *reactea.Ctx, msg tea.Msg, mounts []diskmodel.Mount,
) (tea.Cmd, bool) {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok || click.Button != tea.MouseLeft {
		return nil, false
	}

	_, y, inside := reactea.Mouse(ctx, msg)
	if !inside {
		return nil, false
	}

	index := f.cursor.Offset + y - f.top
	if y < f.top || index >= len(mounts) {
		return nil, false
	}

	f.cursor.Selected = index

	if f.open == nil {
		return nil, true
	}

	return f.open(ctx, mounts[index].Path), true
}

// Fixed-width columns are allocated from the right; the mount path is flexible.
const (
	sizeColumn  = 10
	usedColumn  = 10
	availColumn = 10
	barColumn   = 12
	percColumn  = 6
	typeColumn  = 9
)

func (f *filesystems) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	mounts := f.list()
	if len(mounts) == 0 {
		return f.theme.Style(theme.Dim).Render(" nothing mounted that holds storage")
	}

	f.cursor.Clamp(len(mounts))

	var (
		detail  = f.detail(width, mounts[f.cursor.Selected])
		visible = min(max(1, height-1-len(detail)), len(mounts))
		lines   = make([]string, 0, height)
	)

	f.cursor.Reveal(visible, len(mounts))

	lines = append(lines, heading(f.theme, width, f.header(width)))
	f.top = len(lines)

	for i := range visible {
		index := f.cursor.Offset + i
		if index >= len(mounts) {
			break
		}

		lines = append(lines, f.row(width, mounts[index], index == f.cursor.Selected && ctx.Focused()))
	}

	lines = append(lines, detail...)

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (f *filesystems) header(width int) string {
	if width < 76 {
		fixed := usedColumn + barColumn + percColumn + 4

		return " " + fit("MOUNT", max(6, width-fixed)) + " " +
			fitRight("USED", usedColumn) + " " + fit("", barColumn) + " " +
			fitRight("USE", percColumn)
	}

	fixed := sizeColumn + usedColumn + availColumn + barColumn + percColumn + typeColumn + 7

	return " " + fit("MOUNT", max(6, width-fixed)) + " " +
		fit("TYPE", typeColumn) + " " +
		fitRight("SIZE", sizeColumn) + " " +
		fitRight("USED", usedColumn) + " " +
		fitRight("FREE", availColumn) + " " +
		fit("", barColumn) + " " +
		fitRight("USE", percColumn)
}

func (f *filesystems) row(width int, mount diskmodel.Mount, selected bool) string {
	if width < 76 {
		return f.compactRow(width, mount, selected)
	}

	fixed := sizeColumn + usedColumn + availColumn + barColumn + percColumn + typeColumn + 7

	text := func(s string) string { return s }
	dim := func(s string) string { return f.theme.Style(theme.Dim).Render(s) }

	if selected {
		return f.theme.Style(theme.Selected).Width(width).Render(" " +
			fit(safe.Text(mount.Path), max(6, width-fixed)) + " " +
			fit(safe.Text(mount.FSType), typeColumn) + " " +
			fitRight(size(mount.Total), sizeColumn) + " " +
			fitRight(size(mount.Used()), usedColumn) + " " +
			fitRight(size(mount.Available), availColumn) + " " +
			plainBar(mount.Usage()/100, barColumn) + " " +
			fitRight(percent(mount.Usage()), percColumn))
	}

	return " " + text(fit(safe.Text(mount.Path), max(6, width-fixed))) + " " +
		dim(fit(safe.Text(mount.FSType), typeColumn)) + " " +
		dim(fitRight(size(mount.Total), sizeColumn)) + " " +
		text(fitRight(size(mount.Used()), usedColumn)) + " " +
		dim(fitRight(size(mount.Available), availColumn)) + " " +
		bar(f.theme, mount.Usage()/100, barColumn) + " " +
		f.theme.Style(f.theme.Threshold(mount.Usage(), 75, 90)).Render(fitRight(percent(mount.Usage()), percColumn))
}

func (f *filesystems) compactRow(width int, mount diskmodel.Mount, selected bool) string {
	fixed := usedColumn + barColumn + percColumn + 4
	path := fit(safe.Text(mount.Path), max(6, width-fixed))
	used := fitRight(size(mount.Used()), usedColumn)
	usage := fitRight(percent(mount.Usage()), percColumn)

	if selected {
		return f.theme.Style(theme.Selected).Width(width).Render(" " + path + " " + used + " " +
			plainBar(mount.Usage()/100, barColumn) + " " + usage)
	}

	return " " + path + " " + f.theme.Style(theme.Text).Render(used) + " " +
		bar(f.theme, mount.Usage()/100, barColumn) + " " +
		f.theme.Style(f.theme.Threshold(mount.Usage(), 75, 90)).Render(usage)
}

func (f *filesystems) detail(width int, mount diskmodel.Mount) []string {
	label := func(s string) string { return f.theme.Style(theme.Dim).Render(s) }

	options := safe.Text(mount.Opts)
	options = uirender.Ellipsize(options, max(0, width/2))

	return []string{
		"",
		" " + label("device  ") + safe.Text(mount.Device),
		" " + label("options ") + options,
		" " + label("inodes  ") + inodes(mount),
	}
}

// inodes distinguishes dynamic inode allocation from an exhausted inode pool.
func inodes(mount diskmodel.Mount) string {
	if mount.Inodes == 0 {
		return "— (this filesystem allocates them as it goes)"
	}

	return series.Count.Format(float64(mount.Inodes-mount.InodesFree), 0, series.Auto) +
		" of " + series.Count.Format(float64(mount.Inodes), 0, series.Auto) +
		"  " + percent(mount.InodeUsage())
}

func size(bytes uint64) string { return series.Bytes.Format(float64(bytes), 1, series.Auto) }
