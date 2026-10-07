package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

// tabSpan separates the clickable tab bounds from its underline bounds.
type tabSpan struct {
	start, width           int
	labelStart, labelWidth int
}

func (r *Root) digit(mode page.Mode) string {
	keys := r.keys.Keys(keymap.Global, keymap.Mode)
	if mode.Slot < 0 || mode.Slot >= len(keys) {
		return ""
	}

	return keys[mode.Slot]
}

func (r *Root) tabSpans(width int) []tabSpan {
	var (
		spans = make([]tabSpan, len(r.modes))
		left  int
		right int
	)

	measure := func(mode page.Mode) (number, label int) {
		digit := r.digit(mode)
		if digit != "" {
			number = 1 + lipgloss.Width(digit)
		}

		return number, 2 + lipgloss.Width(mode.Title)
	}

	for _, mode := range r.modes {
		number, label := measure(mode)

		if mode.Right {
			right += number + label
		} else {
			left += number + label
		}
	}

	status := r.rightSideFor(max(0, width-left-right))
	at := map[bool]int{false: 0, true: max(left, width-right-lipgloss.Width(status))}

	for i, mode := range r.modes {
		number, label := measure(mode)

		spans[i] = tabSpan{
			start: at[mode.Right], width: number + label,
			labelStart: at[mode.Right] + number, labelWidth: label,
		}

		at[mode.Right] += number + label
	}

	return spans
}

func (r *Root) renderTabs(ctx *reactea.Ctx) string {
	var (
		number = r.theme.Style(theme.Dim)
		idle   = r.theme.Style(theme.Tab)
		here   = r.theme.Style(theme.TabActive)
		spans  = r.tabSpans(ctx.Width())
		left   strings.Builder
		right  strings.Builder
		start  int
		span   int
	)

	for i, mode := range r.modes {
		style := idle

		if mode.Holds(ctx.Route()) {
			style, start, span = here, spans[i].labelStart, spans[i].labelWidth
		}

		into := &left
		if mode.Right {
			into = &right
		}

		key := r.digit(mode)
		if key != "" {
			into.WriteString(number.Render(" " + key))
		}
		into.WriteString(style.Render(fmt.Sprintf(" %s ", mode.Title)))
	}

	leftWidth, rightWidth := 0, 0
	for i, mode := range r.modes {
		if mode.Right {
			rightWidth += spans[i].width
		} else {
			leftWidth += spans[i].width
		}
	}
	right.WriteString(r.theme.Style(theme.Dim).Render(
		r.rightSideFor(max(0, ctx.Width()-leftWidth-rightWidth))))

	bar := left.String()
	if gap := ctx.Width() - lipgloss.Width(bar) - lipgloss.Width(right.String()); gap > 0 {
		bar += strings.Repeat(" ", gap)
	}

	bar += right.String()

	return lipgloss.NewStyle().MaxWidth(ctx.Width()).Render(bar) + "\n" +
		r.rule(ctx.Width(), start, span)
}

func (r *Root) clickedTab(ctx *reactea.Ctx, msg tea.Msg) int {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok || click.Button != tea.MouseLeft {
		return -1
	}

	x, y, ok := reactea.MouseAt(msg)
	if !ok || y != 0 {
		return -1
	}

	for i, span := range r.tabSpans(ctx.Width()) {
		if x >= span.start && x < span.start+span.width {
			return i
		}
	}

	return -1
}

func (r *Root) rule(width, start, span int) string {
	if width <= 0 {
		return ""
	}

	span = min(span, max(0, width-start))
	thin := r.theme.Style(theme.Border)

	return thin.Render(strings.Repeat("─", min(start, width))) +
		r.theme.Style(theme.BorderActive).Render(strings.Repeat("━", span)) +
		thin.Render(strings.Repeat("─", max(0, width-start-span)))
}

func (r *Root) renderHints(ctx *reactea.Ctx) string {
	for _, problem := range []string{r.configProblem(), r.pageError()} {
		if problem != "" {
			return r.theme.Style(theme.Critical).
				Width(ctx.Width()).MaxWidth(ctx.Width()).Render(" " + safe.Text(problem))
		}
	}

	if note := r.pageNote(); note != "" {
		return r.theme.Style(theme.Own).
			Width(ctx.Width()).MaxWidth(ctx.Width()).Render(" " + safe.Text(note))
	}

	var (
		key   = lipgloss.NewStyle().Bold(true)
		dim   = lipgloss.NewStyle().Faint(true)
		parts []string
	)

	for _, hint := range r.hints() {
		parts = append(parts, key.Render(hint.Key)+dim.Render(" "+hint.What))
	}

	return lipgloss.NewStyle().MaxWidth(ctx.Width()).
		Render(" " + strings.Join(parts, dim.Render("  ")))
}

func (r *Root) hints() []page.Hint {
	if hinter, ok := r.pages.Current().(page.Hinter); ok {
		return hinter.Hints()
	}

	return nil
}

func (r *Root) pageNote() string {
	return page.NoteOf(r.pages.Current())
}

func (r *Root) configProblem() string {
	if r.env.Live == nil {
		return ""
	}

	return r.env.Live.Problem()
}

func (r *Root) pageError() string {
	return page.ErrorOf(r.pages.Current())
}

// rightSideFor drops complete status items from lowest priority until they fit.
func (r *Root) rightSideFor(width int) string {
	parts := make([]string, 0, 3)

	if src, _ := r.env.View.Filter(); src != "" {
		parts = append(parts, "filter:"+safe.Text(src))
	}

	if count, name := actionableProblems(r.env.Scheduler.Problems()); count > 0 {
		if count == 1 {
			parts = append(parts, "no "+name+" readings")
		} else {
			parts = append(parts, fmt.Sprintf("%d collectors unavailable", count))
		}
	}

	if rss, ok := r.env.Store.Last("self.rss"); ok {
		self := "self " + series.Bytes.Format(rss.Value, 0, series.Auto)

		if cpu, ok := r.env.Store.Last("self.cpu"); ok {
			self += " " + series.Percent.Format(cpu.Value, 1, series.Auto)
		}

		parts = append(parts, self)
	}

	for len(parts) > 0 {
		out := "  " + strings.Join(parts, "  ") + " "
		if lipgloss.Width(out) <= width {
			return out
		}

		parts = parts[:len(parts)-1]
	}

	return ""
}

func actionableProblems(all map[string]collect.Availability) (count int, name string) {
	for candidate, availability := range all {
		if availability.State != collect.NoHardware {
			count++
			if count == 1 {
				name = candidate
			}
		}
	}

	return count, name
}
