package conf

import (
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/Hayao0819/hytop/internal/errors"
)

// NeedsSetup reports whether an ordinary launch has no configuration layer.
func NeedsSetup(s Sources) (bool, error) {
	if s.User == "" || s.Extra != "" || s.Profile != "" {
		return false, nil
	}

	for _, path := range []string{s.System, s.User} {
		if path == "" {
			continue
		}

		_, err := os.Stat(path)
		switch {
		case err == nil:
			return false, nil
		case os.IsNotExist(err):
			continue
		default:
			return false, errors.Wrapf(err, "checking %s", path)
		}
	}

	return true, nil
}

type setupFile struct {
	General    *setupGeneral    `toml:"general,omitempty"`
	Appearance *setupAppearance `toml:"appearance,omitempty"`
	Graphs     *setupGraphs     `toml:"graphs,omitempty"`
	Processes  *setupProcesses  `toml:"processes,omitempty"`
}

type setupGeneral struct {
	Interval *Duration `toml:"interval,omitempty"`
	Span     *Duration `toml:"span,omitempty"`
}

type setupAppearance struct {
	Contrast   *string `toml:"contrast,omitempty"`
	Background *string `toml:"background,omitempty"`
}

type setupGraphs struct {
	Glyphs *string `toml:"glyphs,omitempty"`
	Colors *string `toml:"colors,omitempty"`
}

type setupProcesses struct {
	Tree   *bool `toml:"tree,omitempty"`
	Kernel *bool `toml:"kernel,omitempty"`
}

// SaveSetup creates a minimal user layer without replacing an existing file.
func SaveSetup(path string, base, selected Config) error {
	if err := selected.Validate(); err != nil {
		return err
	}

	contents := setupChanges(base, selected)
	out, err := toml.Marshal(contents)
	if err != nil {
		return errors.Wrap(err, "writing the initial configuration")
	}
	out = append([]byte("# configured by hytop setup\n\n"), out...)

	temporary, err := stage(path, out)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary) }()

	if err := os.Link(temporary, path); err != nil {
		return errors.Wrapf(err, "creating %s", path)
	}

	return nil
}

func setupChanges(base, selected Config) setupFile {
	var out setupFile

	if selected.General.Interval != base.General.Interval || selected.General.Span != base.General.Span {
		out.General = &setupGeneral{}
		if selected.General.Interval != base.General.Interval {
			out.General.Interval = &selected.General.Interval
		}
		if selected.General.Span != base.General.Span {
			out.General.Span = &selected.General.Span
		}
	}

	if selected.Appearance != base.Appearance {
		out.Appearance = &setupAppearance{}
		if selected.Appearance.Contrast != base.Appearance.Contrast {
			out.Appearance.Contrast = &selected.Appearance.Contrast
		}
		if selected.Appearance.Background != base.Appearance.Background {
			out.Appearance.Background = &selected.Appearance.Background
		}
	}

	if selected.Graphs.Glyphs != base.Graphs.Glyphs || selected.Graphs.Colors != base.Graphs.Colors {
		out.Graphs = &setupGraphs{}
		if selected.Graphs.Glyphs != base.Graphs.Glyphs {
			out.Graphs.Glyphs = &selected.Graphs.Glyphs
		}
		if selected.Graphs.Colors != base.Graphs.Colors {
			out.Graphs.Colors = &selected.Graphs.Colors
		}
	}

	if selected.Processes.Tree != base.Processes.Tree || selected.Processes.Kernel != base.Processes.Kernel {
		out.Processes = &setupProcesses{}
		if selected.Processes.Tree != base.Processes.Tree {
			out.Processes.Tree = &selected.Processes.Tree
		}
		if selected.Processes.Kernel != base.Processes.Kernel {
			out.Processes.Kernel = &selected.Processes.Kernel
		}
	}

	return out
}
