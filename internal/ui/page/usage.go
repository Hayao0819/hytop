package page

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"
	"github.com/Hayao0819/reactea/v2/state"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

type usage struct {
	reactea.BasicComponent

	theme *theme.Theme
	keys  *keymap.Map
	build func() Scanner

	scanner   Scanner
	walk      context.Context
	root      string
	cursor    selection.Cursor
	note      string
	err       string
	pending   elevationRequest
	elevation state.Resource[elevationRequest]
}

type elevationOperation int

const (
	elevateNone elevationOperation = iota
	elevateDelete
	elevateScan
)

type elevationRequest struct {
	operation elevationOperation
	path      string
}

func newUsage(env Env, build func() Scanner) *usage {
	return &usage{theme: env.Theme, keys: env.Keys, build: build, root: home()}
}

// At changes the scan root.
func (u *usage) At(root string) {
	u.root = root
	u.cursor.Reset()
	u.note, u.err = "", ""

	if u.scanner != nil && u.walk != nil {
		u.scanner.Start(u.walk, root)
	}
}

func home() string {
	if dir, err := os.UserHomeDir(); err == nil {
		return dir
	}

	return "/"
}

// Init starts a scan bound to the page scope.
func (u *usage) Init(ctx *reactea.Ctx) tea.Cmd {
	if u.build == nil {
		return nil
	}

	u.scanner = u.build()
	if u.scanner == nil {
		return nil
	}

	u.walk = ctx.Context()

	u.scanner.Start(u.walk, u.root)

	return nil
}

func (u *usage) Hints() []Hint {
	if u.elevation.Loading() || u.scanner != nil && !u.scanner.CanElevate() {
		return u.keys.Hints(keymap.Usage, keymap.Elevate)
	}
	return u.keys.Hints(keymap.Usage)
}

func (u *usage) first(action keymap.Action) string {
	keys := u.keys.Keys(keymap.Usage, action)
	if len(keys) == 0 {
		return "?"
	}

	return keys[0]
}

func (u *usage) Note() string { return u.note }

func (u *usage) Error() string {
	if u.err != "" {
		return u.err
	}

	if u.scanner != nil {
		problem := u.scanner.Scan().Err
		if strings.Contains(strings.ToLower(problem), "permission denied") {
			if u.scanner.CanElevate() {
				return problem + " — press " + u.first(keymap.Elevate) + " to retry as administrator"
			}
			return problem + " — administrator retry is unavailable on this platform"
		}

		return problem
	}

	return ""
}

func (u *usage) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if u.elevation.Handle(msg) {
		return u.finishElevation()
	}
	if answer, ok := msg.(modal.Result[DeleteChoice]); ok {
		return u.delete(ctx, answer)
	}
	if answer, ok := msg.(modal.Result[PasswordChoice]); ok {
		return u.elevate(ctx, answer)
	}

	if u.scanner == nil {
		return nil
	}

	entries := u.scanner.Scan().Entries

	switch {
	case u.keys.Is(msg, keymap.Usage, keymap.Down):
		u.cursor.Move(1, len(entries))

	case u.keys.Is(msg, keymap.Usage, keymap.Up):
		u.cursor.Move(-1, len(entries))

	case u.keys.Is(msg, keymap.Usage, keymap.Top):
		u.cursor.Top()

	case u.keys.Is(msg, keymap.Usage, keymap.Bottom):
		u.cursor.Bottom(len(entries))

	case u.keys.Is(msg, keymap.Usage, keymap.Open):
		return u.enter(entries)

	case u.keys.Is(msg, keymap.Usage, keymap.Back):
		return u.leave()

	case u.keys.Is(msg, keymap.Usage, keymap.Rescan):
		u.rescan(false)

	case u.keys.Is(msg, keymap.Usage, keymap.Remeasure):
		u.rescan(true)

	case u.keys.Is(msg, keymap.Usage, keymap.Elevate):
		if u.elevation.Loading() || !u.scanner.CanElevate() {
			return nil
		}
		return u.askPassword(ctx, elevationRequest{operation: elevateScan, path: u.root})

	case u.keys.Is(msg, keymap.Usage, keymap.Delete):
		return u.remove(ctx, entries)
	}

	return nil
}

func (u *usage) askPassword(ctx *reactea.Ctx, request elevationRequest) tea.Cmd {
	u.pending = request
	reason := "Read inaccessible information under " + safe.Text(request.path)
	if request.operation == elevateDelete {
		reason = "Delete " + safe.Text(request.path)
	}

	return modal.PushAt(ctx, newPasswordDialog(u.theme, u.keys, reason), modal.Centered(64, 10))
}

func (u *usage) elevate(ctx *reactea.Ctx, answer modal.Result[PasswordChoice]) tea.Cmd {
	if !answer.Ok() {
		u.err = safe.Text(answer.Err.Error())
		return nil
	}

	request := u.pending
	u.pending = elevationRequest{}
	password := answer.Value.Password
	u.err, u.note = "", "working as administrator…"

	return u.elevation.Load(ctx, func(walk context.Context) (elevationRequest, error) {
		defer func() { password = "" }()

		var err error
		switch request.operation {
		case elevateDelete:
			err = u.scanner.RemoveElevated(walk, request.path, password)
		case elevateScan:
			err = u.scanner.RestartElevated(walk, request.path, password)
		}

		return request, err
	})
}

func (u *usage) finishElevation() tea.Cmd {
	if err := u.elevation.Err(); err != nil {
		u.err, u.note = safe.Text(err.Error()), ""
		return nil
	}

	done := u.elevation.Value()
	u.err = ""
	if done.operation == elevateDelete {
		u.note = "deleted " + safe.Text(done.path) + " as administrator"
		u.scanner.Restart(u.walk, u.root)
	} else {
		u.note = "read " + safe.Text(done.path) + " as administrator"
	}

	return nil
}

func (u *usage) enter(entries []diskmodel.Entry) tea.Cmd {
	if u.cursor.Selected >= len(entries) || !entries[u.cursor.Selected].Dir {
		return nil
	}

	u.open(entries[u.cursor.Selected].Path)

	return nil
}

func (u *usage) leave() tea.Cmd {
	parent := filepath.Dir(u.root)
	if parent == u.root {
		return nil
	}

	u.open(parent)

	return nil
}

func (u *usage) open(path string) {
	u.root = path
	u.cursor.Reset()
	u.note, u.err = "", ""

	u.scanner.Start(u.walk, path)
}

func (u *usage) rescan(full bool) {
	u.note, u.err = "", ""

	if full {
		u.scanner.Restart(u.walk, u.root)

		return
	}

	u.scanner.Start(u.walk, u.root)
}

// remove binds confirmation to the displayed path rather than the live cursor.
func (u *usage) remove(ctx *reactea.Ctx, entries []diskmodel.Entry) tea.Cmd {
	if u.cursor.Selected >= len(entries) {
		return nil
	}

	target := entries[u.cursor.Selected]
	u.err, u.note = "", ""

	return modal.PushAt(ctx, newDeleteDialog(u.theme, u.keys, target), modal.Centered(62, 11))
}

func (u *usage) delete(ctx *reactea.Ctx, answer modal.Result[DeleteChoice]) tea.Cmd {
	if !answer.Ok() {
		u.err = safe.Text(answer.Err.Error())

		return nil
	}

	if err := u.scanner.Remove(answer.Value.Path); err != nil {
		if errors.Is(err, os.ErrPermission) && u.scanner.CanElevate() {
			return u.askPassword(ctx, elevationRequest{operation: elevateDelete, path: answer.Value.Path})
		}
		u.err, u.note = safe.Text(err.Error()), ""

		return nil
	}

	u.err = ""
	u.note = "deleted " + safe.Text(answer.Value.Path)

	// Discard cached totals after deletion.
	u.scanner.Restart(u.walk, u.root)

	return nil
}

func (u *usage) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	if u.scanner == nil {
		return u.theme.Style(theme.Dim).Render(" no scanner")
	}

	scan := u.scanner.Scan()

	var (
		lines   = []string{u.title(width, scan), heading(u.theme, width, u.header(width))}
		visible = max(1, height-len(lines))
	)

	if len(scan.Entries) == 0 {
		lines = append(lines, " "+u.theme.Style(theme.Dim).Render(u.empty()))
	}

	u.cursor.Clamp(len(scan.Entries))
	u.cursor.Reveal(visible, len(scan.Entries))

	largest := uint64(0)
	if len(scan.Entries) > 0 {
		largest = scan.Entries[0].Size
	}

	for i := range visible {
		index := u.cursor.Offset + i
		if index >= len(scan.Entries) {
			break
		}

		lines = append(lines, u.row(width, scan.Entries[index], largest, index == u.cursor.Selected && ctx.Focused()))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (u *usage) empty() string {
	if u.scanner.Running() {
		return "counting…"
	}

	return "nothing here"
}

func (u *usage) title(width int, scan diskmodel.Scan) string {
	state := u.theme.Style(theme.Own).Render("done")
	if !scan.Done {
		state = u.theme.Style(theme.Warn).Render("counting…")
	}

	if scan.Reused > 0 {
		state += u.theme.Style(theme.Dim).
			Render(fmt.Sprintf("  %d reused", scan.Reused))
	}

	left := " " + u.theme.Style(theme.Heading).Render(safe.Text(scan.Root))
	right := u.theme.Style(theme.Text).Render(size(scan.Total)) + "  " +
		u.theme.Style(theme.Dim).Render(fmt.Sprintf("%s items", series.Count.Format(float64(scan.Items), 0, series.Auto))) +
		"  " + state + " "

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left, width)
	}

	return left + strings.Repeat(" ", gap) + right
}

const (
	usageSize = 10
	usageBar  = 20
	usageItem = 9
)

func (u *usage) header(width int) string {
	return " " + fit("NAME", max(8, width-usageSize-usageBar-usageItem-5)) + " " +
		fitRight("SIZE", usageSize) + " " + fit("", usageBar) + " " + fitRight("ITEMS", usageItem)
}

func (u *usage) row(width int, entry diskmodel.Entry, largest uint64, selected bool) string {
	name := safe.Text(entry.Name)
	if entry.Dir {
		name += "/"
	}

	if entry.Mount {
		name += "  (another filesystem)"
	}

	var fraction float64
	if largest > 0 {
		fraction = float64(entry.Size) / float64(largest)
	}

	nameWidth := max(8, width-usageSize-usageBar-usageItem-5)

	if selected {
		return u.theme.Style(theme.Selected).Width(width).Render(" " +
			fit(name, nameWidth) + " " +
			fitRight(size(entry.Size), usageSize) + " " +
			plainBar(fraction, usageBar) + " " +
			fitRight(fmt.Sprint(entry.Items), usageItem))
	}

	token := theme.Text
	if entry.Dir {
		token = theme.Heading
	}

	filled, rest := chart.Gauge(fraction, usageBar, render.Block)

	return " " + u.theme.Style(token).Render(fit(name, nameWidth)) + " " +
		fitRight(size(entry.Size), usageSize) + " " +
		u.theme.Style(theme.DiskWrite).Render(filled) +
		u.theme.Style(theme.Dim).Render(rest) + " " +
		u.theme.Style(theme.Dim).Render(fitRight(fmt.Sprint(entry.Items), usageItem))
}
