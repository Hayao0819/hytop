package page

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"
	"github.com/Hayao0819/reactea/v2/state"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

type drives struct {
	reactea.BasicComponent

	theme   *theme.Theme
	keys    *keymap.Map
	list    Drives
	refresh RefreshDrives

	cursor selection.Cursor
	note   string
	err    string
	load   state.Resource[struct{}]
}

func newDrives(env Env, list Drives, refresh RefreshDrives) *drives {
	return &drives{theme: env.Theme, keys: env.Keys, list: list, refresh: refresh}
}

func (d *drives) Hints() []Hint {
	if d.refresh == nil || d.load.Loading() || !d.needsPrivilege() {
		return d.keys.Hints(keymap.Drives, keymap.Elevate)
	}

	return d.keys.Hints(keymap.Drives)
}

func (d *drives) Note() string  { return d.note }
func (d *drives) Error() string { return d.err }

func (d *drives) needsPrivilege() bool {
	for _, device := range d.list() {
		reason := strings.ToLower(device.Reason)
		if strings.Contains(reason, "privilege") || strings.Contains(reason, "permission") {
			return true
		}
	}

	return false
}

func (d *drives) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if d.load.Handle(msg) {
		if err := d.load.Err(); err != nil {
			d.err, d.note = safe.Text(err.Error()), ""
		} else {
			d.err, d.note = "", "SMART information refreshed as administrator"
		}

		return nil
	}
	if answer, ok := msg.(modal.Result[PasswordChoice]); ok {
		if !answer.Ok() || d.refresh == nil {
			return nil
		}
		password := answer.Value.Password
		refresh := d.refresh
		d.err, d.note = "", "refreshing SMART information…"

		return d.load.Load(ctx, func(read context.Context) (struct{}, error) {
			defer func() { password = "" }()

			return struct{}{}, refresh(read, password)
		})
	}

	total := len(d.list())

	switch {
	case d.keys.Is(msg, keymap.Disk, keymap.Down):
		d.cursor.Move(1, total)
	case d.keys.Is(msg, keymap.Disk, keymap.Up):
		d.cursor.Move(-1, total)
	case d.keys.Is(msg, keymap.Disk, keymap.Top):
		d.cursor.Top()
	case d.keys.Is(msg, keymap.Disk, keymap.Bottom):
		d.cursor.Bottom(total)
	case d.keys.Is(msg, keymap.Drives, keymap.Elevate):
		if d.refresh != nil && !d.load.Loading() && d.needsPrivilege() {
			return modal.PushAt(ctx,
				newPasswordDialog(d.theme, d.keys, "Read SMART information from the drives"),
				modal.Centered(64, 10))
		}
	}

	return nil
}

func (d *drives) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	devices := d.list()
	if len(devices) == 0 {
		return d.theme.Style(theme.Dim).Render(" no drives found under /sys/block")
	}

	d.cursor.Clamp(len(devices))
	visible := min(len(devices), max(1, (height-1)/3))
	d.cursor.Reveal(visible, len(devices))

	lines := []string{heading(d.theme, width, " "+fit("DRIVE", 10)+" "+
		fit("MODEL", max(10, width-46))+" "+
		fitRight("SIZE", 10)+"  "+fit("KIND", 8)+" "+fit("HEALTH", 8))}

	for i := range visible {
		index := d.cursor.Offset + i
		lines = append(lines, d.row(width, devices[index], index == d.cursor.Selected && ctx.Focused()))
	}

	lines = append(lines, d.detail(width, devices[d.cursor.Selected], height-len(lines))...)

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (d *drives) row(width int, device diskmodel.Device, selected bool) string {
	line := " " + fit(safe.Text(device.Name), 10) + " " +
		fit(safe.Text(device.Model), max(10, width-46)) + " " +
		fitRight(size(device.Size), 10) + "  " +
		fit(device.Kind(), 8) + " " +
		fit(device.Health.String(), 8)

	if selected {
		return d.theme.Style(theme.Selected).Width(width).Render(line)
	}

	if device.Health == diskmodel.Failing {
		return d.theme.Style(theme.Critical).Render(line)
	}

	return line
}

func (d *drives) detail(width int, device diskmodel.Device, room int) []string {
	if room <= 0 {
		return nil
	}

	label := func(s string) string { return d.theme.Style(theme.Dim).Render(s) }
	lines := []string{"", " " + d.theme.Style(theme.Heading).Render(safe.Text(device.Name)+"  "+safe.Text(device.Model))}

	facts := []string{
		label("bus ") + orDash(device.Bus),
		label("serial ") + orDash(device.Serial),
		label("firmware ") + orDash(device.Firmware),
		label("scheduler ") + orDash(device.Sched),
	}

	lines = append(lines, " "+strings.Join(facts, label("   ")))

	if device.Reason != "" {
		lines = append(lines, "", " "+d.theme.Style(theme.Warn).Render(safe.Text(device.Reason)))

		return lines[:min(len(lines), room)]
	}

	lines = append(lines, " "+strings.Join(d.health(device), label("   ")))

	if len(device.Attributes) == 0 {
		return lines
	}

	lines = append(lines, "", heading(d.theme, width, " "+fit("ID", 5)+fit("ATTRIBUTE", max(20, width-46))+
		fitRight("VALUE", 7)+fitRight("WORST", 7)+fitRight("THRESH", 8)+"  "+fit("RAW", 18)))

	for _, attribute := range device.Attributes {
		if len(lines) >= room {
			break
		}

		lines = append(lines, d.attribute(width, attribute))
	}

	return lines[:min(len(lines), room)]
}

func (d *drives) health(device diskmodel.Device) []string {
	label := func(s string) string { return d.theme.Style(theme.Dim).Render(s) }

	token := theme.Own
	if device.Health == diskmodel.Failing {
		token = theme.Critical
	}

	facts := []string{label("health ") + d.theme.Style(token).Render(device.Health.String())}

	if device.Temperature > 0 {
		facts = append(facts, label("temperature ")+
			d.theme.Style(d.theme.Threshold(device.Temperature, 55, 70)).
				Render(series.Celsius.Format(device.Temperature, 0, series.Auto)))
	}

	if device.PowerOnTime > 0 {
		facts = append(facts, label("powered on ")+
			fmt.Sprintf("%.0f h", device.PowerOnTime.Hours()))
	}

	if device.PowerCycles > 0 {
		facts = append(facts, label("cycles ")+fmt.Sprint(device.PowerCycles))
	}

	if device.Wear >= 0 {
		facts = append(facts, label("wear ")+
			d.theme.Style(d.theme.Threshold(device.Wear, 70, 90)).Render(percent(device.Wear)))
	}

	if device.Written > 0 {
		facts = append(facts, label("written ")+size(device.Written))
	}

	return facts
}

func (d *drives) attribute(width int, attribute diskmodel.Attribute) string {
	line := " " + fit(fmt.Sprint(attribute.ID), 5) +
		fit(safe.Text(attribute.Name), max(20, width-46)) +
		fitRight(fmt.Sprint(attribute.Value), 7) +
		fitRight(fmt.Sprint(attribute.Worst), 7) +
		fitRight(fmt.Sprint(attribute.Threshold), 8) + "  " +
		safe.Text(attribute.Raw)

	if attribute.Failing {
		return d.theme.Style(theme.Critical).Render(line)
	}

	return line
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}

	return safe.Text(value)
}
