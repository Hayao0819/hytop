// Package app assembles the running UI.
package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/layout"
	"github.com/Hayao0819/reactea/v2/modal"
	"github.com/Hayao0819/reactea/v2/router"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/errors"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/version"
)

// Live provides the current hot-reloaded configuration.
type Live interface {
	Config() conf.Config
	Generation() uint64
	Problem() string
}

// Env contains application dependencies that reactea.Ctx does not carry.
type Env struct {
	Store     *store.Store
	View      *store.ViewState
	Registry  *series.Registry
	Scheduler *collect.Scheduler

	Caps   render.Caps
	Config conf.Config
	Live   Live

	Interfaces    func() []string
	GPUs          func() []int
	Batteries     func() []int
	Units         func() []unitmodel.Unit
	FollowLog     func() page.LogFollower
	Mounts        func() []diskmodel.Mount
	Drives        func() []diskmodel.Device
	RefreshDrives func(context.Context, string) error
	Scanner       func() page.Scanner

	// A nil Save disables saving from the settings screen.
	Save     func(conf.Config) error
	SavePath string

	// OnConfig propagates UI and file changes to collection.
	OnConfig func(conf.Config)
}

type Root struct {
	reactea.Wrapper

	env        Env
	config     conf.Config
	generation uint64
	caps       render.Caps
	dark       *bool
	theme      *theme.Theme
	keys       *keymap.Map
	pending    *conf.Config

	modes      []page.Mode
	pages      *router.Component
	graphs     *page.Graphs
	processes  *page.Processes
	containers *page.Containers
	services   *page.Services
	storage    *page.Storage
	settings   *page.Settings
	dashboards map[string]*page.Dashboard

	// home retains the last route visited in each mode.
	home map[string]string

	stop func()
}

var chrome = lipgloss.NewStyle().Padding(1, 1, 0, 0)

// Refresh requests a periodic application update.
type Refresh time.Time

func New(env Env) *Root {
	if env.Interfaces == nil {
		env.Interfaces = func() []string { return nil }
	}

	if env.Units == nil {
		env.Units = func() []unitmodel.Unit { return nil }
	}
	if env.GPUs == nil {
		env.GPUs = func() []int { return nil }
	}
	if env.Batteries == nil {
		env.Batteries = func() []int { return nil }
	}

	root := &Root{env: env, config: env.Config, home: map[string]string{}}

	if env.Live != nil {
		root.config, root.generation = env.Live.Config(), env.Live.Generation()
	}
	root.setProcessConfig(root.config)

	root.build()

	root.pages = router.NewWithRoutes(root.routes())

	body := layout.Column(
		layout.Fixed(2, reactea.Func(root.renderTabs)),
		layout.Grow(1, layout.Framed(chrome, root.pages)).Focusable(),
		layout.Fixed(1, reactea.Func(root.renderHints)),
	)

	root.Wrapper = reactea.Wrap(body)

	return root
}

// build reconstructs pages; persistent UI state remains in ViewState.
func (r *Root) build() {
	r.caps = r.config.Caps(r.env.Caps)
	r.theme = theme.BuildWith(r.caps, r.config.Palette(), r.config.ThemeOptions(r.dark))
	r.keys = r.config.Keymap()

	pageEnv := page.Env{
		Store:      r.env.Store,
		View:       r.env.View,
		Registry:   r.env.Registry,
		Caps:       r.caps,
		Theme:      r.theme,
		Keys:       r.keys,
		Span:       time.Duration(r.config.General.Span),
		Columns:    r.config.Columns(),
		Interfaces: r.env.Interfaces,
		GPUs:       r.env.GPUs,
		Batteries:  r.env.Batteries,
	}

	r.graphs = page.NewGraphs(pageEnv)
	r.processes = page.NewProcesses(pageEnv, r.env.FollowLog)
	r.containers = page.NewContainers(pageEnv)
	r.services = page.NewServices(pageEnv, r.env.Units, r.env.FollowLog)
	r.storage = page.NewStorage(pageEnv, r.env.Mounts, r.env.Drives, r.env.RefreshDrives, r.env.Scanner)
	r.settings = page.NewSettings(pageEnv, r.settingItems()...)
	r.settings.Path = r.env.SavePath
	r.dashboards = make(map[string]*page.Dashboard, len(r.config.Pages))
	r.modes = page.Modes()

	for _, spec := range r.config.Pages {
		route := dashboardRoute(spec.Name)
		r.dashboards[route] = page.NewDashboard(pageEnv, spec)

		mode := page.Mode{Title: page.DashboardTitle(spec), Slot: -1, Home: route}
		at := len(r.modes)
		for i, existing := range r.modes {
			if existing.Right {
				at = i

				break
			}
		}
		r.modes = append(r.modes, page.Mode{})
		copy(r.modes[at+1:], r.modes[at:])
		r.modes[at] = mode
	}

	if r.env.Save != nil {
		r.settings.Save = func() error { return r.env.Save(r.config) }
	}
}

func dashboardRoute(name string) string { return "/dashboards/" + name }

func (r *Root) routes() router.Routes {
	routes := router.Routes{
		"/graphs/*rest": func(router.Params) reactea.Component { return r.graphs },
		"/processes":    func(router.Params) reactea.Component { return r.processes },
		"/containers":   func(router.Params) reactea.Component { return r.containers },
		"/services":     func(router.Params) reactea.Component { return r.services },
		"/disk/*rest":   func(router.Params) reactea.Component { return r.storage },
		"/settings":     func(router.Params) reactea.Component { return r.settings },
		"default":       func(router.Params) reactea.Component { return r.graphs },
	}

	for route, dashboard := range r.dashboards {
		dashboard := dashboard
		routes[route] = func(router.Params) reactea.Component { return dashboard }
	}

	return routes
}

// apply defers page replacement until the current Update returns.
func (r *Root) apply(c conf.Config) { r.pending = &c }

func (r *Root) rebuild(ctx *reactea.Ctx) tea.Cmd {
	previous, next := r.config, *r.pending
	r.config, r.pending = next, nil
	r.syncProcessConfig(previous, next)

	row := r.settings.Selected()

	r.build()
	r.settings.Select(row)
	r.pages.SetRoutes(r.routes(), router.PreserveCurrent)

	if r.env.OnConfig != nil {
		r.env.OnConfig(r.config)
	}

	commands := []tea.Cmd{r.pages.Reload(ctx)}
	if previous.Appearance.Background != conf.Auto && next.Appearance.Background == conf.Auto {
		commands = append(commands, tea.RequestBackgroundColor)
	}
	if !r.routeKnown(ctx.Route()) {
		commands = append(commands, ctx.SetRoute(r.modes[0].Home))
	}

	return tea.Batch(commands...)
}

func (r *Root) routeKnown(route string) bool {
	for _, mode := range r.modes {
		if mode.Holds(route) {
			return true
		}
	}

	return false
}

func (r *Root) syncProcessConfig(previous, next conf.Config) {
	if previous.Processes.Sort != next.Processes.Sort ||
		previous.Processes.Descending != next.Processes.Descending {
		r.env.View.SetSortOrder(store.SortKey(next.Processes.Sort), next.Processes.Descending)
	}
	if previous.Processes.Tree != next.Processes.Tree {
		r.env.View.SetTree(next.Processes.Tree)
	}
	if previous.Processes.Kernel != next.Processes.Kernel {
		r.env.View.SetKernel(next.Processes.Kernel)
	}
	if processFilterChanged(previous, next) {
		source, expr, err := next.Filter(next.Processes.Filter)
		if err == nil {
			r.env.View.SetFilter(source, expr)
		}
	}
}

func processFilterChanged(previous, next conf.Config) bool {
	if previous.Processes.Filter != next.Processes.Filter {
		return true
	}
	if next.Processes.Filter == "" {
		return false
	}

	before, _, _ := previous.Filter(previous.Processes.Filter)
	after, _, _ := next.Filter(next.Processes.Filter)

	return before != after
}

func (r *Root) setProcessConfig(c conf.Config) {
	r.env.View.SetSortOrder(store.SortKey(c.Processes.Sort), c.Processes.Descending)
	r.env.View.SetTree(c.Processes.Tree)
	r.env.View.SetKernel(c.Processes.Kernel)
	if source, expr, err := c.Filter(c.Processes.Filter); err == nil {
		r.env.View.SetFilter(source, expr)
	}
}

// OnQuit registers cleanup for the root component's scope.
func (r *Root) OnQuit(stop func()) { r.stop = stop }

func (r *Root) Init(ctx *reactea.Ctx) tea.Cmd {
	if r.stop != nil {
		ctx.OnDestroy(r.stop)
	}

	commands := []tea.Cmd{
		r.Wrapper.Init(ctx),
		reactea.SetMouseMode(tea.MouseModeCellMotion),
		tick(r.interval()),
	}
	if r.config.Appearance.Background == conf.Auto {
		commands = append(commands, tea.RequestBackgroundColor)
	}

	return tea.Batch(commands...)
}

func (r *Root) interval() time.Duration { return time.Duration(r.config.General.Interval) }

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return Refresh(t) })
}

func (r *Root) Update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	cmd := r.update(ctx, msg)

	if r.pending == nil {
		return cmd
	}

	return tea.Batch(cmd, r.rebuild(ctx))
}

func (r *Root) update(ctx *reactea.Ctx, msg tea.Msg) tea.Cmd {
	if background, ok := msg.(tea.BackgroundColorMsg); ok && r.config.Appearance.Background == conf.Auto {
		dark := background.IsDark()
		if r.dark == nil || *r.dark != dark {
			r.dark = &dark
			r.apply(r.config)
		}

		return nil
	}

	if _, ok := msg.(Refresh); ok {
		r.catchUp()

		return tea.Batch(tick(r.interval()), r.Wrapper.Update(ctx, msg))
	}

	if r.keys.Is(msg, keymap.Global, keymap.ForceQuit) {
		return tea.Quit
	}

	if changed, ok := msg.(reactea.RouteChangedMsg); ok {
		r.remember(changed.To)
	}

	if ctx.InputCaptured() {
		return r.Wrapper.Update(ctx, msg)
	}

	if i := r.clickedTab(ctx, msg); i >= 0 {
		return r.enter(ctx, i)
	}

	if cmd, handled := r.globalKey(ctx, msg); handled {
		return cmd
	}

	return r.Wrapper.Update(ctx, msg)
}

// catchUp applies watcher changes on the UI goroutine.
func (r *Root) catchUp() {
	if r.env.Live == nil {
		return
	}

	if generation := r.env.Live.Generation(); generation != r.generation {
		r.generation = generation

		r.apply(r.env.Live.Config())
	}
}

func (r *Root) globalKey(ctx *reactea.Ctx, msg tea.Msg) (tea.Cmd, bool) {
	switch {
	case r.keys.Is(msg, keymap.Global, keymap.Quit):
		return tea.Quit, true

	case r.keys.Is(msg, keymap.Global, keymap.Help):
		return modal.PushAt(ctx, newHelp(r.keys), modal.Centered(72, 30)), true

	case r.keys.Is(msg, keymap.Global, keymap.About):
		return modal.PushAt(ctx, newAbout(r.theme, r.keys, version.Current()), aboutPlacement(ctx)), true

	case r.keys.Is(msg, keymap.Global, keymap.NextMode):
		return r.stepMode(ctx, 1), true

	case r.keys.Is(msg, keymap.Global, keymap.PrevMode):
		return r.stepMode(ctx, -1), true
	}

	if i := r.keys.Index(msg, keymap.Global, keymap.Mode); i >= 0 {
		for at, mode := range r.modes {
			if mode.Slot == i {
				return r.enter(ctx, at), true
			}
		}
	}

	return nil, false
}

func (r *Root) remember(route string) {
	for _, mode := range r.modes {
		if mode.Holds(route) {
			r.home[mode.Home] = route

			return
		}
	}
}

func (r *Root) enter(ctx *reactea.Ctx, index int) tea.Cmd {
	mode := r.modes[index]

	if last, ok := r.home[mode.Home]; ok {
		return ctx.SetRoute(last)
	}

	return ctx.SetRoute(mode.Home)
}

func (r *Root) stepMode(ctx *reactea.Ctx, by int) tea.Cmd {
	current := 0

	for i, mode := range r.modes {
		if mode.Holds(ctx.Route()) {
			current = i
		}
	}

	return r.enter(ctx, (current+by+len(r.modes))%len(r.modes))
}

// Run wires the scheduler to the program and blocks until the user quits.
func Run(ctx context.Context, env Env) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	root := New(env)
	root.OnQuit(cancel)

	go env.Scheduler.Run(ctx)

	// Seed the route before Bubble Tea draws its initial frame.
	program := reactea.New(modal.New(root),
		reactea.WithRoute("/graphs/"+root.config.Graphs.Device),
		reactea.WithAltScreen(),
		reactea.WithWindowTitle("hytop"),
	)

	if err := program.Run(); err != nil {
		return errors.Wrap(err, "running the terminal program")
	}

	return nil
}
