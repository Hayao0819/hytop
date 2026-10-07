package proctable

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type value struct {
	text  func(*Widget, Row) string
	token func(*Widget, Row) theme.Token
}

func plain(text func(Row) string) value {
	return value{text: func(_ *Widget, r Row) string { return text(r) }}
}

func dim(text func(Row) string) value {
	return value{
		text:  func(_ *Widget, r Row) string { return text(r) },
		token: func(*Widget, Row) theme.Token { return theme.Dim },
	}
}

func bytesOf(read func(*procmodel.Process) uint64) value {
	return dim(func(r Row) string {
		return series.Bytes.Format(float64(read(r.Proc)), 0, series.Auto)
	})
}

func percentOf(read func(*procmodel.Process) float64, warn, critical float64) value {
	return value{
		text: func(_ *Widget, r Row) string { return fmt.Sprintf("%.1f", read(r.Proc)) },
		token: func(w *Widget, r Row) theme.Token {
			return w.theme.Threshold(read(r.Proc), warn, critical)
		},
	}
}

var values = map[string]value{
	"pid":     dim(func(r Row) string { return fmt.Sprint(r.Proc.PID) }),
	"ppid":    dim(func(r Row) string { return fmt.Sprint(r.Proc.PPID) }),
	"pgid":    dim(func(r Row) string { return fmt.Sprint(r.Proc.PGID) }),
	"uid":     dim(func(r Row) string { return fmt.Sprint(r.Proc.UID) }),
	"threads": dim(func(r Row) string { return fmt.Sprint(r.Proc.Threads) }),
	"fds":     dim(func(r Row) string { return fmt.Sprint(r.Proc.FDs) }),
	"nice":    dim(func(r Row) string { return fmt.Sprint(r.Proc.Nice) }),
	"unit":    dim(func(r Row) string { return r.Proc.Unit }),
	"cgroup":  dim(func(r Row) string { return r.Proc.Cgroup }),

	"container": plain(func(r Row) string { return r.Proc.Container }),
	"vm":        plain(func(r Row) string { return r.Proc.VM }),
	"exe":       plain(func(r Row) string { return r.Proc.Exe }),
	"name":      plain(func(r Row) string { return r.Proc.Name }),

	"vsz":      bytesOf(func(p *procmodel.Process) uint64 { return p.VSZ }),
	"io.read":  bytesOf(func(p *procmodel.Process) uint64 { return p.IORd }),
	"io.write": bytesOf(func(p *procmodel.Process) uint64 { return p.IOWr }),
	"gpu.mem":  bytesOf(func(p *procmodel.Process) uint64 { return p.GPUMB }),

	"gpu.util": percentOf(func(p *procmodel.Process) float64 { return p.GPUPc }, 20, 60),
	"npu.util": percentOf(func(p *procmodel.Process) float64 { return p.NPUPc }, 20, 60),
	"cpu":      percentOf(func(p *procmodel.Process) float64 { return p.CPU }, 20, 60),

	"user": {
		text:  func(_ *Widget, r Row) string { return r.Proc.User },
		token: func(w *Widget, r Row) theme.Token { return w.userToken(r.Proc) },
	},
	"state": {
		text:  func(_ *Widget, r Row) string { return r.Proc.State },
		token: func(_ *Widget, r Row) theme.Token { return stateToken(r.Proc.State) },
	},
	"rss": {
		text: func(_ *Widget, r Row) string {
			return series.Bytes.Format(float64(r.Proc.RSS), 0, series.Auto)
		},
		token: func(w *Widget, r Row) theme.Token {
			return w.theme.Threshold(memoryShare(w.store, r.Proc), 10, 25)
		},
	},
	"command": {
		text:  func(_ *Widget, r Row) string { return r.Prefix + marker(r) + procmodel.Command(r.Proc) },
		token: func(w *Widget, r Row) theme.Token { return w.commandToken(r.Proc) },
	},
	"cmdline": {
		text:  func(_ *Widget, r Row) string { return r.Prefix + marker(r) + r.Proc.Cmdline },
		token: func(w *Widget, r Row) theme.Token { return w.commandToken(r.Proc) },
	},
}

func (w *Widget) cellText(column procmodel.Column, r Row) string {
	got, ok := values[column.Key]
	if !ok || got.text == nil {
		return ""
	}

	// Process fields are untrusted terminal input; styles are applied afterward.
	return safe.Text(got.text(w, r))
}

func (w *Widget) cellToken(column procmodel.Column, r Row) theme.Token {
	got, ok := values[column.Key]
	if !ok || got.token == nil {
		return theme.Text
	}

	return got.token(w, r)
}

func (w *Widget) Render(ctx *reactea.Ctx) string {
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		return ""
	}

	rows := w.Rows()
	w.scrollIntoView(ctx)
	offset := min(max(w.view.Offset(), 0), max(0, len(rows)-1))

	empty := ""
	if note := w.empty(rows); note != "" {
		empty = w.theme.Style(theme.Dim).Render(" " + note)
	}

	return render.Table(height,
		w.theme.Style(theme.TableHeader).Width(width).Render(w.headerLine(width)),
		empty, len(rows), offset, func(index int) string {
			if index == w.view.Selected() && ctx.Focused() {
				return w.theme.Style(theme.Selected).Width(width).Render(w.plainRow(rows[index], width))
			}

			return w.row(rows[index], width)
		})
}

func (w *Widget) empty(rows []Row) string {
	if len(rows) > 0 {
		return ""
	}

	if !w.view.Kernel() {
		return "nothing matches — and the kernel threads are hidden; H shows them"
	}

	if src, _ := w.view.Filter(); src != "" {
		return "nothing matches " + src
	}

	return "no processes"
}

func (w *Widget) headerLine(width int) string {
	by, descending := w.view.Sort()

	return w.line(width, func(column procmodel.Column) string {
		title := column.Title

		if column.Sort != "" && store.SortKey(column.Sort) == by {
			if descending {
				title += "▼"
			} else {
				title += "▲"
			}
		}

		return title
	}, nil)
}

// line clips whole columns to one terminal row measured in display cells.
func (w *Widget) line(
	width int, text func(procmodel.Column) string, paint func(procmodel.Column) theme.Token,
) string {
	var (
		b    strings.Builder
		used int
	)

	for i, column := range w.columns {
		room := width - used
		if room <= 1 {
			if i < len(w.columns) && room == 1 {
				b.WriteString(w.theme.Style(theme.Dim).Render("›"))
			}

			break
		}

		drawn := cell(text(column), column.Width, column.Right, room)
		used += lipgloss.Width(drawn) + 1

		if paint != nil {
			drawn = w.theme.Style(paint(column)).Render(drawn)
		}

		b.WriteString(drawn)
		b.WriteByte(' ')
	}

	return b.String()
}

func (w *Widget) plainRow(r Row, width int) string {
	return w.line(width, func(column procmodel.Column) string { return w.cellText(column, r) }, nil)
}

func (w *Widget) row(r Row, width int) string {
	return w.line(width,
		func(column procmodel.Column) string { return w.cellText(column, r) },
		func(column procmodel.Column) theme.Token { return w.cellToken(column, r) })
}

func (w *Widget) userToken(p *procmodel.Process) theme.Token {
	switch {
	case p.User == w.self && w.self != "":
		return theme.Own
	case p.User == "root":
		return theme.Root
	default:
		return theme.Dim
	}
}

func (w *Widget) commandToken(p *procmodel.Process) theme.Token {
	if p.Kthread {
		return theme.Kernel
	}

	return theme.Text
}

func stateToken(state string) theme.Token {
	switch state {
	case "R":
		return theme.Own
	case "D":
		return theme.Critical
	case "Z":
		return theme.Warn
	default:
		return theme.Dim
	}
}

func memoryShare(s *store.Store, p *procmodel.Process) float64 {
	total, ok := s.Last("mem.total")
	if !ok || total.Value <= 0 {
		return 0
	}

	return 100 * float64(p.RSS) / total.Value
}

func marker(r Row) string {
	switch {
	case r.Collapsed && r.Children:
		return "+ "
	case r.Prefix != "":
		return " "
	default:
		return ""
	}
}

func cell(value string, width int, right bool, remaining int) string {
	if width == 0 || width >= remaining {
		width = max(0, remaining-1)
	}

	if right {
		return render.Right(value, width)
	}

	return render.Left(value, width)
}
