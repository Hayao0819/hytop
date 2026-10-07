package page

import (
	"context"
	"fmt"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

type logPane struct {
	theme *theme.Theme

	open func() LogFollower

	unit string
	log  LogFollower
	stop func()

	// room retains the rendered height used to clamp offset.
	offset int
	room   int
}

func newLogPane(t *theme.Theme, open func() LogFollower) *logPane {
	if open == nil {
		open = func() LogFollower { return nil }
	}

	return &logPane{theme: t, open: open}
}

func (p *logPane) Unit() string { return p.unit }

// Follow starts a scope-bound journal reader unless unit is already active.
func (p *logPane) Follow(ctx *reactea.Ctx, unit string) {
	if p.log != nil && p.unit == unit {
		return
	}

	p.Close()

	p.unit = unit
	if unit == "" {
		return
	}

	p.log = p.open()
	if p.log == nil {
		return
	}

	follow, stop := context.WithCancel(ctx.Context())
	p.stop = stop

	p.log.Follow(follow, unit)
}

func (p *logPane) Close() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}

	p.unit, p.log, p.offset = "", nil, 0
}

// Scroll moves through the journal while keeping a full window when possible.
func (p *logPane) Scroll(by int) {
	var total int
	if p.log != nil {
		total = len(p.log.Entries())
	}

	p.offset = min(max(p.offset+by, 0), max(0, total-max(1, p.room)))
}

// Lines returns the visible journal window, newest last and padded to height.
func (p *logPane) Lines(width, height int) []string {
	p.room = height

	if height <= 0 || p.log == nil {
		return nil
	}

	if err := p.log.Err(); err != nil {
		return []string{p.theme.Style(theme.Critical).Render(" " + err.Error())}
	}

	entries := p.log.Entries()

	offset := min(p.offset, max(0, len(entries)-height))
	end := len(entries) - offset
	start := max(0, end-height)

	lines := make([]string, 0, height)

	for _, entry := range entries[start:end] {
		// Journal messages are untrusted terminal input.
		line := fmt.Sprintf(" %s  %s", entry.Time.Format("15:04:05"), safe.Text(entry.Message))

		lines = append(lines, p.theme.Style(logToken(entry)).Render(render.Clip(line, width)))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return lines
}

func logToken(entry unitmodel.LogEntry) theme.Token {
	switch {
	case entry.Error():
		return theme.Critical
	case entry.Warning():
		return theme.Warn
	default:
		return theme.Text
	}
}
