package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
)

// Sources are snapshot readers discovered during background initialization.
// Nil readers return empty snapshots until the corresponding source is ready.
type Sources struct {
	Interfaces func() []string
	GPUs       func() []int
	Batteries  func() []int
	Units      func() []unitmodel.Unit
	Mounts     func() []diskmodel.Mount
}

func (s *Sources) interfaces() []string {
	if s.Interfaces != nil {
		return s.Interfaces()
	}
	return nil
}

func (s *Sources) gpus() []int {
	if s.GPUs != nil {
		return s.GPUs()
	}
	return nil
}

func (s *Sources) batteries() []int {
	if s.Batteries != nil {
		return s.Batteries()
	}
	return nil
}

func (s *Sources) units() []unitmodel.Unit {
	if s.Units != nil {
		return s.Units()
	}
	return nil
}

func (s *Sources) mounts() []diskmodel.Mount {
	if s.Mounts != nil {
		return s.Mounts()
	}
	return nil
}

type initializedMsg struct {
	sources Sources
	err     error
}

// initialize runs as a Bubble Tea command, after Init has returned to rendering.
func (r *Root) initialize(ctx context.Context) tea.Cmd {
	initialize, scheduler, sources := r.env.Initialize, r.env.Scheduler, r.sources

	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return initializedMsg{err: err}
		}
		if initialize != nil {
			var err error
			sources, err = initialize(ctx)
			if err != nil {
				return initializedMsg{err: err}
			}
		}
		if err := ctx.Err(); err != nil {
			return initializedMsg{err: err}
		}

		go scheduler.Run(ctx)

		return initializedMsg{sources: sources}
	}
}
