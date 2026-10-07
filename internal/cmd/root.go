// Package cmd is the command line.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/collect/filesystem"
	"github.com/Hayao0819/hytop/internal/collect/journal"
	"github.com/Hayao0819/hytop/internal/collect/smart"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

type devices struct {
	interfaces func() []string
	gpus       func() []int
	batteries  func() []int
	units      func() []unitmodel.Unit
	mounts     func() []diskmodel.Mount
}

type options struct {
	procRoot string
	sysRoot  string
	runRoot  string
	config   string
	profile  string
	interval time.Duration
	span     time.Duration
	filter   string
	trace    bool
}

func New(version string) *cobra.Command {
	var opts options

	root := &cobra.Command{
		Use:           "hytop",
		Short:         "A terminal system monitor",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts)
		},
	}

	flags := root.PersistentFlags()
	flags.StringVar(&opts.procRoot, "proc", "/proc", "path to the proc filesystem")
	flags.StringVar(&opts.sysRoot, "sys", "/sys", "path to the sys filesystem")
	flags.StringVar(&opts.runRoot, "run", "/run", "path to the run directory, where udev keeps its database")
	flags.StringVarP(&opts.config, "config", "c", "", "read this file instead of the usual ones")
	flags.StringVarP(&opts.profile, "profile", "p", "", "also read profiles/<name>.toml")
	flags.DurationVar(&opts.interval, "interval", time.Second, "how often to collect")
	flags.DurationVar(&opts.span, "span", time.Minute, "how much history a graph shows")
	flags.StringVarP(&opts.filter, "filter", "f", "", "a filter to start with, by name or expression")
	flags.BoolVar(&opts.trace, "trace", false, "print a stack trace when something fails")

	root.AddCommand(newFilterCmd(), newConfigCmd(), newKeysCmd(), newPrivilegedCmd())

	return root
}

// Execute runs the command and returns its process exit code.
func Execute(version string) int {
	root := New(version)

	if err := root.Execute(); err != nil {
		traced, _ := root.PersistentFlags().GetBool("trace")

		if traced {
			fmt.Fprintf(os.Stderr, "hytop: %s\n", errors.Details(err))
		} else {
			fmt.Fprintf(os.Stderr, "hytop: %v\n", err)
			fmt.Fprintln(os.Stderr, "hytop: run again with --trace for a stack trace")
		}

		return 1
	}

	return 0
}

func run(cmd *cobra.Command, opts options) error {
	sources := conf.Discover(opts.config, opts.profile, nil)
	detected := render.Detect(nil)

	proceed, err := initialSetup(cmd.Context(), sources, detected)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	watcher, err := conf.NewWatcher(sources, overrides(cmd, opts))
	if err != nil {
		return err
	}

	settings := watcher.Config()

	state := store.NewViewState()

	// Allocate lazy tiers for any span a hot reload may select.
	memory := store.NewWithPolicies(metric.ResolutionsFor(conf.MaxSpan), store.Policy{
		Pattern: "battery.*.capacity",
		Resolutions: []metric.Resolution{
			{Interval: time.Second, Retention: 10 * time.Minute},
			{Interval: 10 * time.Second, Retention: 2 * time.Hour},
			{Interval: 5 * time.Minute, Retention: 7 * 24 * time.Hour},
		},
	})
	scheduler := collect.NewScheduler(memory)

	// Collector intervals follow the active, possibly unsaved configuration.
	var running atomic.Pointer[conf.Config]

	running.Store(&settings)

	registry, found, err := register(scheduler, func() conf.Config { return *running.Load() }, opts)
	if err != nil {
		return err
	}
	if err := watcher.AddValidator(func(c conf.Config) error { return c.ValidateSeries(registry) }); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()

	// SMART runs separately because waking a sleeping drive may take seconds.
	drives := smart.NewReader(opts.sysRoot, "/dev")

	go func() {
		for {
			drives.Refresh(ctx)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()

	// Bound initial collection so a stalled source cannot block startup.
	primed, done := context.WithTimeout(ctx, 5*time.Second)
	scheduler.Prime(primed, 2, min(time.Duration(settings.General.Interval), 700*time.Millisecond))
	done()

	savePath := opts.config
	if savePath == "" {
		if dir := conf.UserDir(nil); dir != "" {
			savePath = filepath.Join(dir, "config.toml")
		}
	}

	var save func(conf.Config) error

	if savePath != "" {
		save = func(c conf.Config) error { return conf.Save(savePath, c) }
	}

	elevated, elevateErr := elevator()
	if elevateErr != nil {
		return elevateErr
	}
	elevated.procRoot = opts.procRoot
	elevated.sysRoot = opts.sysRoot

	var refreshDrives func(context.Context, string) error
	if elevated.Available() {
		refreshDrives = func(ctx context.Context, password string) error {
			devices, err := elevated.Drives(ctx, password)
			if err == nil {
				drives.ReplaceElevated(devices)
			}

			return err
		}
	}

	return app.Run(ctx, app.Env{
		Store:         memory,
		View:          state,
		Registry:      registry,
		Scheduler:     scheduler,
		Caps:          detected,
		Config:        settings,
		Live:          watcher,
		Interfaces:    found.interfaces,
		GPUs:          found.gpus,
		Batteries:     found.batteries,
		Units:         found.units,
		FollowLog:     func() page.LogFollower { return journal.NewReader(running.Load().Logs.Lines) },
		Mounts:        found.mounts,
		Drives:        drives.Devices,
		RefreshDrives: refreshDrives,
		Scanner: func() page.Scanner {
			scanner := diskusage.NewScanner(func() ([]diskmodel.Mount, error) {
				return filesystem.Boundaries(opts.procRoot)
			})
			if elevated.Available() {
				scanner.SetElevator(elevated)
			}
			return scanner
		},
		Save:     save,
		SavePath: savePath,
		OnConfig: func(c conf.Config) { running.Store(&c) },
	})
}

func initialSetup(ctx context.Context, sources conf.Sources, detected render.Caps) (bool, error) {
	needed, err := conf.NeedsSetup(sources)
	if err != nil || !needed {
		return !needed, err
	}

	base, err := sources.Load()
	if err != nil {
		return false, err
	}

	return app.RunSetup(ctx, base, detected, sources.User, func(selected conf.Config) error {
		return conf.SaveSetup(sources.User, base, selected)
	})
}

func flagged(cmd *cobra.Command) options {
	var opts options

	flags := cmd.Flags()
	opts.procRoot, _ = flags.GetString("proc")
	opts.sysRoot, _ = flags.GetString("sys")
	opts.runRoot, _ = flags.GetString("run")
	opts.interval, _ = flags.GetDuration("interval")
	opts.span, _ = flags.GetDuration("span")
	opts.filter, _ = flags.GetString("filter")

	return opts
}

func overrides(cmd *cobra.Command, opts options) func(*conf.Config) {
	flags := cmd.Flags()

	return func(c *conf.Config) {
		if flags.Changed("interval") {
			c.General.Interval = conf.Duration(opts.interval)
		}

		if flags.Changed("span") {
			c.General.Span = conf.Duration(opts.span)
		}

		if flags.Changed("filter") {
			c.Processes.Filter = opts.filter
		}
	}
}
