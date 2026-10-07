package app_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/modal"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
)

func fixture(t *testing.T) (*reactea.App, *store.Store, *store.ViewState) {
	t.Helper()

	return fixtureWith(t, nil)
}

func fixtureWith(t *testing.T, adjust func(*app.Env)) (*reactea.App, *store.Store, *store.ViewState) {
	t.Helper()

	memory := store.New(metric.DefaultResolutions())

	memory.WriteProcesses([]procmodel.Process{
		{PID: 1, PPID: 0, Name: "systemd", User: "root", Cmdline: "/sbin/init", Threads: 1, CPU: 0.1, RSS: 10 << 20},
		{PID: 10, PPID: 1, Name: "sshd", User: "root", Cmdline: "sshd", Threads: 1, CPU: 0.2, RSS: 20 << 20, Unit: "sshd.service"},
		{PID: 11, PPID: 10, Name: "bash", User: "hayao", Cmdline: "-bash", Threads: 1, CPU: 3, RSS: 30 << 20, Unit: "session-2.scope"},
		{PID: 12, PPID: 11, Name: "vim", User: "hayao", Cmdline: "vim x.go", Threads: 2, CPU: 1, RSS: 40 << 20},
		{PID: 20, PPID: 1, Name: "firefox", User: "hayao", Cmdline: "firefox", Threads: 9, CPU: 40, RSS: 900 << 20},
		{PID: 2, PPID: 0, Name: "kthreadd", User: "root", Threads: 1, Kthread: true},
	})

	now := time.Now()

	for i := range 30 {
		at := now.Add(time.Duration(i-30) * time.Second)

		memory.WriteSamples([]metric.Sample{
			{Key: "cpu.total.usage", Value: float64(i) * 3, Time: at},
			{Key: "cpu.total.freq", Value: 3.6e9, Time: at},
			{Key: "cpu.package.temp", Value: 55, Time: at},
			{Key: "system.uptime", Value: 90000, Time: at},
			{Key: "cpu.core.0.usage", Value: float64(i) * 2, Time: at},
			{Key: "cpu.core.1.usage", Value: float64(i), Time: at},
			{Key: "mem.usage", Value: 40, Time: at},
			{Key: "mem.used", Value: 8 << 30, Time: at},
			{Key: "mem.available", Value: 12 << 30, Time: at},
			{Key: "mem.cached", Value: 4 << 30, Time: at},
			{Key: "mem.total", Value: 20 << 30, Time: at},
			{Key: "swap.used", Value: 0, Time: at},
			{Key: "swap.total", Value: 4 << 30, Time: at},
			{Key: "psi.cpu.some.avg10", Value: 0.2, Time: at},
			{Key: "psi.memory.some.avg10", Value: 0, Time: at},
			{Key: "proc.count", Value: 6, Time: at},
			{Key: "proc.running", Value: 1, Time: at},
			{Key: "proc.sleeping", Value: 5, Time: at},
			{Key: "proc.threads", Value: 15, Time: at},
			{Key: "proc.fds", Value: 200, Time: at},
			{Key: "load.1", Value: 0.5, Time: at},
			{Key: "load.5", Value: 0.4, Time: at},
			{Key: "load.15", Value: 0.3, Time: at},
			{Key: "self.rss", Value: 12 << 20, Time: at},
			{Key: "diskio.total.read", Value: float64(i * (1 << 20)), Time: at},
			{Key: "diskio.total.write", Value: float64(i * (1 << 19)), Time: at},
			{Key: "net.total.rx", Value: float64(i) * 1000, Time: at},
			{Key: "net.total.tx", Value: float64(i) * 500, Time: at},
		})
	}

	memory.WriteFacts(map[string]string{
		"cpu.model":          "Test CPU 9000",
		"cpu.sockets":        "1",
		"cpu.cores":          "8",
		"cpu.logical":        "16",
		"cpu.base_speed":     "3.60 GHz",
		"cpu.virtualisation": "AMD-V",
		"cpu.cache.l1d":      "512 KiB",
		"cpu.cache.l2":       "8 MiB",
		"cpu.cache.l3":       "32 MiB",
	})

	var registry series.Registry

	view := store.NewViewState()

	settings := conf.Default()
	settings.General.Interval = conf.Duration(time.Millisecond)

	env := app.Env{
		Store:      memory,
		View:       view,
		Registry:   &registry,
		Scheduler:  collect.NewScheduler(memory),
		Caps:       render.Caps{Glyphs: render.Braille, Colors: render.Ansi256},
		Config:     settings,
		Interfaces: func() []string { return []string{"eth0", "wlan0"} },
		Units: func() []unitmodel.Unit {
			return []unitmodel.Unit{
				{Name: "sshd.service", Description: "OpenSSH server", Load: "loaded", Active: "active", Sub: "running"},
				{Name: "broken.service", Description: "Something wrong", Load: "loaded", Active: "failed", Sub: "failed"},
			}
		},
	}

	if adjust != nil {
		adjust(&env)
	}

	program := reactea.New(modal.New(app.New(env)), reactea.WithSize(100, 28), reactea.WithRoute("/graphs/cpu"))

	_ = program.Init()

	return program, memory, view
}

func reachable(memory *store.Store) []page.GraphSpec {
	return page.Available(page.GraphSpecs(), func(key series.Key) bool {
		_, ok := memory.Last(key)

		return ok
	})
}

func mode(t *testing.T, program *reactea.App, title string) {
	t.Helper()

	for _, m := range page.Modes() {
		if m.Title == title {
			press(t, program, keymap.Default().Keys(keymap.Global, keymap.Mode)[m.Slot])

			return
		}
	}
	if runtime.GOOS != "linux" && (title == "Services" || title == "Containers") {
		t.Skipf("%s mode is Linux-only", title)
	}

	t.Fatalf("no mode called %q", title)
}

func device(t *testing.T, program *reactea.App, memory *store.Store, title string) {
	t.Helper()

	mode(t, program, "Graphs")

	specs := reachable(memory)

	for range len(specs) + 1 {
		for _, spec := range specs {
			if spec.Title == title && program.Route() == spec.Route {
				return
			}
		}

		press(t, program, "j")
	}

	t.Fatalf("could not reach the %q graph, stopped at %s", title, program.Route())
}

func press(t *testing.T, program *reactea.App, keys ...string) {
	t.Helper()

	testkit.SendKeys(program, keys...)
}

func plain(program *reactea.App) string { return testkit.Plain(program) }

func TestTheShellHasTabsARailAndAGraph(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	view := plain(program)

	for _, want := range []string{"1 Graphs", "2 Processes"} {
		if !strings.Contains(view, want) {
			t.Errorf("mode %q is missing:\n%s", want, view)
		}
	}

	for _, want := range []string{"CPU", "Memory", "Disk", "Network"} {
		if !strings.Contains(view, want) {
			t.Errorf("the device rail is missing %q:\n%s", want, view)
		}
	}

	if !strings.ContainsAny(view, "⣿⣾⣴⣠⢀") {
		t.Errorf("no graph was drawn:\n%s", view)
	}

	if lines := strings.Split(program.View().Content, "\n"); len(lines) != 28 {
		t.Errorf("drew %d lines into a 28-row box", len(lines))
	}
}

func TestEveryModeFitsSmallTerminalSizes(t *testing.T) {
	t.Parallel()

	for _, caps := range []render.Caps{
		{Glyphs: render.Braille, Colors: render.Ansi256},
		{Glyphs: render.Block, Colors: render.Ansi16},
		{Glyphs: render.ASCII, Colors: render.Mono},
	} {
		program, _, _ := fixtureWith(t, func(env *app.Env) { env.Caps = caps })

		for _, size := range [][2]int{{1, 1}, {8, 3}, {24, 6}, {40, 8}, {80, 24}} {
			program.Send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})

			for _, available := range page.Modes() {
				mode(t, program, available.Title)
				raw := program.View().Content
				width, height := lipgloss.Size(raw)
				if width > size[0] || height > size[1] {
					t.Fatalf("%s rendered %dx%d into %dx%d", available.Title, width, height, size[0], size[1])
				}
				if !utf8.ValidString(raw) {
					t.Fatalf("%s rendered invalid UTF-8 at %dx%d", available.Title, size[0], size[1])
				}
			}
		}
	}
}

func TestNumbersAndTabSwitchPages(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	device(t, program, memory, "Memory")

	if got := plain(program); !strings.Contains(got, "Available") {
		t.Errorf("the memory page did not open:\n%s", got)
	}

	mode(t, program, "Processes")

	if got := plain(program); !strings.Contains(got, "COMMAND") {
		t.Errorf("the process table did not open:\n%s", got)
	}

	press(t, program, "tab")

	if got := plain(program); strings.Contains(got, "COMMAND") {
		t.Errorf("tab did not wrap back to the first page:\n%s", got)
	}
}

func TestAModeRemembersWhereItWasLeft(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	device(t, program, memory, "Memory")

	mode(t, program, "Processes")

	if got := plain(program); !strings.Contains(got, "COMMAND") {
		t.Errorf("the process table did not open:\n%s", got)
	}

	mode(t, program, "Graphs")

	if program.Route() != "/graphs/memory" {
		t.Errorf("came back to %s, want the memory graph", program.Route())
	}
}

func TestTheKeyLineFollowsThePage(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	graphKeys := plain(program)
	if strings.Contains(graphKeys, "subtree") || strings.Contains(graphKeys, "sort") {
		t.Errorf("a graph page advertised the table's keys:\n%s", lastLine(graphKeys))
	}

	mode(t, program, "Processes")

	tableKeys := plain(program)
	for _, want := range []string{"subtree", "sort", "filter"} {
		if !strings.Contains(tableKeys, want) {
			t.Errorf("the table did not advertise %q:\n%s", want, lastLine(tableKeys))
		}
	}

	press(t, program, "/")

	if got := plain(program); !strings.Contains(got, "apply") || strings.Contains(got, "subtree") {
		t.Errorf("the key line did not follow the filter bar:\n%s", lastLine(got))
	}
}

func TestTheGraphRailKeepsAVisibleBoundaryWhenShort(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	got := plain(program)
	if boundaries := strings.Count(got, "┄"); boundaries < len(reachable(memory)) {
		t.Errorf("the short graph rail has only %d boundary cells:\n%s", boundaries, got)
	}
}

func TestClickingTheGraphRailOpensTheDrawnDevice(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)
	if !testkit.ClickText(program, "Memory") {
		t.Fatalf("no Memory entry in the graph rail:\n%s", plain(program))
	}

	if got := program.Route(); got != "/graphs/memory" {
		t.Fatalf("route = %q, want the clicked graph entry", got)
	}
}

func TestOnlyTheLeftButtonOpensAGraphRailEntry(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)
	found := testkit.Find(program, "Memory")
	if len(found) == 0 {
		t.Fatalf("no Memory entry in the graph rail:\n%s", plain(program))
	}

	program.Send(tea.MouseClickMsg{X: found[0].X, Y: found[0].Y, Button: tea.MouseRight})
	if got := program.Route(); got != "/graphs/cpu" {
		t.Fatalf("right click changed the route to %q", got)
	}
}

type unavailableCollector struct {
	name         string
	availability collect.Availability
}

func (c unavailableCollector) Name() string                { return c.name }
func (c unavailableCollector) Check() collect.Availability { return c.availability }
func (unavailableCollector) Collect(context.Context, time.Time) ([]metric.Sample, error) {
	return nil, nil
}

func TestTheStatusBarDoesNotCallAbsentHardwareAFailure(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Scheduler.Every(unavailableCollector{
			name: "optional-gpu",
			availability: collect.Availability{
				State: collect.NoHardware, Reason: "no GPU found",
			},
		}, time.Minute)
	})
	program.Send(tea.WindowSizeMsg{Width: 180, Height: 28})

	if got := plain(program); strings.Contains(got, "optional-gpu") || strings.Contains(got, "collectors unavailable") {
		t.Fatalf("absent optional hardware was shown as a failure:\n%s", got)
	}
}

func TestTheStatusBarStillReportsCollectorFailures(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Scheduler.Every(unavailableCollector{
			name: "broken",
			availability: collect.Availability{
				State: collect.Failed, Reason: "read failed",
			},
		}, time.Minute)
	})
	program.Send(tea.WindowSizeMsg{Width: 180, Height: 28})

	if got := plain(program); !strings.Contains(got, "no broken readings") {
		t.Fatalf("collector failure disappeared from the status bar:\n%s", got)
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")

	return lines[len(lines)-1]
}

func TestTheCPUPageShowsItsReadings(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	device(t, program, memory, "CPU")

	got := plain(program)

	for _, want := range []string{
		"Utilisation", "Speed", "Temperature", "Up time",
		"Processes", "Threads", "Handles",
		"Base speed", "Sockets", "Cores", "Logical processors", "Virtualisation",
		"L1 cache", "L2 cache", "L3 cache",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the CPU page is missing %q:\n%s", want, got)
		}
	}
}

func TestCSwitchesTheCPUPageToPerCorePlots(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	device(t, program, memory, "CPU")

	press(t, program, "c")

	got := plain(program)
	if !strings.Contains(got, "0 ") || !strings.Contains(got, "1 ") {
		t.Errorf("no per-core plots after c:\n%s", got)
	}

	press(t, program, "c")

	if after := plain(program); !strings.Contains(after, "utilisation") {
		t.Errorf("c did not switch back to the single plot:\n%s", after)
	}
}

func TestTheCPUPageShowsTheModelAndFacts(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	device(t, program, memory, "CPU")

	if got := plain(program); !strings.Contains(got, "Test CPU 9000") {
		t.Errorf("the model is missing from the heading:\n%s", got)
	}

	if got := plain(program); !strings.Contains(got, "AMD-V") {
		t.Errorf("virtualisation is missing:\n%s", got)
	}
}

func TestTheRailIsTheOrderTheArrowKeysWalk(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixture(t)

	mode(t, program, "Graphs")

	specs := reachable(memory)

	for i, spec := range specs {
		if program.Route() != spec.Route {
			t.Fatalf("step %d is at %s, want %s", i, program.Route(), spec.Route)
		}

		press(t, program, "j")
	}

	if program.Route() != specs[0].Route {
		t.Errorf("the last step did not wrap to %s, it is at %s", specs[0].Route, program.Route())
	}

	press(t, program, "k")

	if want := specs[len(specs)-1].Route; program.Route() != want {
		t.Errorf("k went to %s, want %s", program.Route(), want)
	}
}

func TestHelpOpensOverThePageAndCloses(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	press(t, program, "?")

	got := plain(program)
	if !strings.Contains(got, "hytop  keys") || !strings.Contains(got, "Processes") {
		t.Errorf("help did not open:\n%s", got)
	}

	if !strings.Contains(got, "1 Graphs") {
		t.Errorf("help covered the page instead of sitting over it:\n%s", got)
	}

	press(t, program, "esc")

	if strings.Contains(plain(program), "hytop  keys") {
		t.Error("help did not close")
	}
}

func TestAboutShowsTheBuildAndProjectLinks(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	press(t, program, "i")

	got := plain(program)
	for _, want := range []string{
		"about hytop",
		"| |__  _   _| |_ ___  _ __",
		"Version",
		"https://github.com/Hayao0819/hytop",
		"@Hayao0819",
		"https://twitter.com/Hayao0819",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("about dialog is missing %q:\n%s", want, got)
		}
	}

	if !strings.Contains(got, "1 Graphs") {
		t.Errorf("about dialog covered the page instead of sitting over it:\n%s", got)
	}

	press(t, program, "q")

	if strings.Contains(plain(program), "https://github.com/Hayao0819/hytop") {
		t.Error("about dialog did not close")
	}
}

func TestAboutScrollsInAShortTerminal(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)
	program.Send(tea.WindowSizeMsg{Width: 72, Height: 8})

	press(t, program, "i")

	if got := plain(program); !strings.Contains(got, "about hytop ·") || !strings.Contains(got, "j/k scroll") {
		t.Errorf("short about dialog has no fixed title or scroll hint:\n%s", got)
	}

	for range 30 {
		press(t, program, "j")
	}

	got := plain(program)
	if !strings.Contains(got, "https://twitter.com/Hayao0819") {
		t.Errorf("the links cannot be reached in a short terminal:\n%s", got)
	}
	if lines := strings.Split(program.View().Content, "\n"); len(lines) != 8 {
		t.Errorf("about dialog drew %d rows into an 8-row terminal", len(lines))
	} else if !strings.Contains(lines[1], "╭") || !strings.Contains(lines[6], "╰") {
		t.Errorf("about dialog did not keep its vertical margin:\n%s", got)
	}
}

func TestTheProcessTableStillSortsAndMoves(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "j", "j")

	if got := view.Selected(); got != 2 {
		t.Errorf("selected = %d after two j, want 2", got)
	}

	press(t, program, "p")

	if by, descending := view.Sort(); by != store.SortPID || !descending {
		t.Errorf("sort = %v %v after p", by, descending)
	}

	press(t, program, "p")

	if _, descending := view.Sort(); descending {
		t.Error("the same sort key twice did not flip the direction")
	}
}

func TestTheTreeSurvivesSorting(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "t")

	if !view.Tree() {
		t.Fatal("t did not turn the tree on")
	}

	got := plain(program)
	if !strings.Contains(got, "├─") && !strings.Contains(got, "└─") {
		t.Fatalf("no tree was drawn:\n%s", got)
	}

	if depthOf(got, "vim") <= depthOf(got, "-bash") {
		t.Errorf("vim is not drawn below bash:\n%s", got)
	}

	press(t, program, "p")

	if after := plain(program); !strings.Contains(after, "├─") && !strings.Contains(after, "└─") {
		t.Errorf("sorting dropped out of the tree:\n%s", after)
	}

	if after := plain(program); depthOf(after, "vim") <= depthOf(after, "-bash") {
		t.Errorf("sorting flattened the tree:\n%s", after)
	}
}

func TestFoldingHidesASubtree(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "t")

	press(t, program, "g", "z")

	got := plain(program)
	if strings.Contains(got, "firefox") {
		t.Errorf("folding the root left its children on screen:\n%s", got)
	}

	if !strings.Contains(got, "+ ") {
		t.Errorf("a folded subtree is not marked:\n%s", got)
	}

	press(t, program, "z")

	if after := plain(program); !strings.Contains(after, "firefox") {
		t.Errorf("unfolding did not bring the children back:\n%s", after)
	}
}

func depthOf(view, command string) int {
	for _, line := range strings.Split(view, "\n") {
		if at := strings.Index(line, command); at >= 0 {
			return len([]rune(line[:at]))
		}
	}

	return -1
}

func TestTheSignalDialogAsksFirst(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "K")

	got := plain(program)
	if !strings.Contains(got, page.Signals[0].Name) || !strings.Contains(got, page.Signals[len(page.Signals)-1].Name) {
		t.Fatalf("the signal dialog did not open:\n%s", got)
	}

	if !strings.Contains(got, "COMMAND") {
		t.Errorf("the dialog covered the table instead of sitting over it:\n%s", got)
	}

	press(t, program, "esc")

	if after := plain(program); strings.Contains(after, page.Signals[len(page.Signals)-1].Name) {
		t.Errorf("esc did not close the dialog:\n%s", after)
	}
}

func TestSignallingWhatIsNotOursSaysSo(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which may signal init")
	}

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "p")

	view.SetSelected(0)

	press(t, program, "K")
	press(t, program, "enter")

	if got := plain(program); !strings.Contains(got, "not yours to signal") &&
		!strings.Contains(got, "had already gone") {
		t.Errorf("no word about the refused signal:\n%s", lastLine(got))
	}
}

func TestTheServiceListPutsFailuresFirst(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Services")

	got := plain(program)
	if !strings.Contains(got, "UNIT") || !strings.Contains(got, "sshd.service") {
		t.Fatalf("the service list did not open:\n%s", got)
	}

	if strings.Index(got, "broken.service") > strings.Index(got, "sshd.service") {
		t.Errorf("the failed unit is not at the top:\n%s", got)
	}
}

func TestOpeningAService(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Services")
	press(t, program, "enter")

	got := plain(program)
	for _, want := range []string{"broken.service", "Description", "Active", "journal"} {
		if !strings.Contains(got, want) {
			t.Errorf("the unit view is missing %q:\n%s", want, got)
		}
	}

	press(t, program, "esc")

	if after := plain(program); !strings.Contains(after, "UNIT") {
		t.Errorf("esc did not go back to the list:\n%s", after)
	}
}

func TestSettingsChangeSomething(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Settings")

	got := plain(program)
	if !strings.Contains(got, "Graph history") || !strings.Contains(got, "Graph style") {
		t.Fatalf("the settings screen did not open:\n%s", got)
	}
	if !strings.Contains(got, "how far back a plot reaches") ||
		strings.Contains(got, "how often every counter is read") {
		t.Fatalf("settings should explain only the selected row:\n%s", got)
	}

	before := plain(program)

	press(t, program, "l")

	if plain(program) == before {
		t.Error("right changed nothing")
	}
}

func TestFilteringCapturesTheKeys(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "/")

	if !program.InputCaptured() {
		t.Fatal("opening the filter did not capture input")
	}

	if program.View().Cursor == nil {
		t.Error("no cursor while typing")
	}

	press(t, program, "q")

	if !program.InputCaptured() {
		t.Error("q quit instead of being typed")
	}

	press(t, program, "esc")

	if program.InputCaptured() {
		t.Error("input stayed captured after esc")
	}

	if src, _ := view.Filter(); src != "" {
		t.Errorf("esc applied the filter anyway: %q", src)
	}
}

func TestFilteringNarrowsTheTable(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")

	press(t, program, "H", "/")

	for _, key := range []string{"k", "t", "h", "r", "e", "a", "d"} {
		press(t, program, key)
	}

	press(t, program, "enter")

	if src, expr := view.Filter(); src != "kthread" || expr == nil {
		t.Fatalf("filter = %q", src)
	}

	got := plain(program)
	if !strings.Contains(got, "kthreadd") || strings.Contains(got, "firefox") {
		t.Errorf("the table did not narrow:\n%s", got)
	}
}

func TestAnEmptyTableSaysWhyItIsEmpty(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "/")

	for _, key := range []string{"k", "t", "h", "r", "e", "a", "d"} {
		press(t, program, key)
	}

	press(t, program, "enter")

	got := plain(program)
	if !strings.Contains(got, "kernel threads are hidden") {
		t.Errorf("the table went blank without saying why:\n%s", got)
	}
}

func TestABadFilterSaysSoAndKeepsTyping(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "/")

	for _, key := range []string{"n", "o", "p", "e"} {
		press(t, program, key)
	}

	press(t, program, "enter")

	if got := plain(program); !strings.Contains(got, "unknown field") {
		t.Errorf("no message about the bad expression:\n%s", got)
	}

	if src, _ := view.Filter(); src != "" {
		t.Errorf("a bad filter was applied: %q", src)
	}

	if !program.InputCaptured() {
		t.Error("a bad expression should leave the bar open")
	}
}

func TestDrillDownAndBack(t *testing.T) {
	t.Parallel()

	program, _, view := fixture(t)

	mode(t, program, "Processes")
	press(t, program, "j", "j", "s")

	src, _ := view.Filter()
	if !strings.HasPrefix(src, "subtree(pid ==") {
		t.Fatalf("s did not drill down: %q", src)
	}

	press(t, program, "esc")

	if src, _ := view.Filter(); src != "" {
		t.Errorf("esc did not step back: %q", src)
	}
}

func TestClickingATabSwitchesMode(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)
	modes := page.Modes()

	for _, m := range modes {
		start, width := tabLabel(t, program, m.Title)

		click(t, program, start+width/2, 0)

		if got := program.Route(); !m.Holds(got) {
			t.Errorf("clicking %s at column %d went to %q", m.Title, start+width/2, got)
		}
	}

	mode(t, program, "Graphs")

	target := modes[len(modes)-2]
	start, _ := tabLabel(t, program, target.Title)

	click(t, program, start-1, 0)

	if got := program.Route(); !target.Holds(got) {
		t.Errorf("clicking the number for %s went to %q", target.Title, got)
	}

	mode(t, program, "Graphs")

	click(t, program, 1, 6)

	if got := program.Route(); got != "/graphs/cpu" {
		t.Errorf("route = %q; a click in the body changed the mode", got)
	}
}

func click(t *testing.T, program *reactea.App, x, y int) {
	t.Helper()

	program.Send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func tabRule(t *testing.T, program *reactea.App) (start, width int) {
	t.Helper()

	rows := strings.Split(plain(program), "\n")
	if len(rows) < 2 {
		t.Fatal("no rule row")
	}

	for i, r := range []rune(rows[1]) {
		if r != '━' {
			continue
		}

		if width == 0 {
			start = i
		}

		width++
	}

	return start, width
}

func tabLabel(t *testing.T, program *reactea.App, title string) (start, width int) {
	t.Helper()

	for _, at := range testkit.Find(program, title) {
		if at.Y == 0 {
			return at.X - 1, lipgloss.Width(title) + 2
		}
	}

	t.Fatalf("no tab called %q in %q", title, strings.Split(plain(program), "\n")[0])

	return 0, 0
}

func TestTheRuleSitsUnderTheTabItMarks(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)

	for _, m := range page.Modes() {
		mode(t, program, m.Title)

		wantStart, wantWidth := tabLabel(t, program, m.Title)
		gotStart, gotWidth := tabRule(t, program)

		if gotStart != wantStart || gotWidth != wantWidth {
			t.Errorf("%s: the rule is at %d..%d, the tab at %d..%d:\n%s",
				m.Title, gotStart, gotStart+gotWidth, wantStart, wantStart+wantWidth,
				plain(program))
		}
	}
}

func TestSettingsSitsApartFromWhatIsWatched(t *testing.T) {
	t.Parallel()

	program, _, _ := fixture(t)
	modes := page.Modes()

	bar := strings.Split(plain(program), "\n")[0]

	at := strings.Index(bar, "Settings")
	if at < 0 {
		t.Fatalf("no settings tab:\n%s", bar)
	}

	var last page.Mode
	for _, watched := range modes {
		if watched.Right {
			continue
		}

		watchedAt := strings.Index(bar, watched.Title)
		if watchedAt < 0 {
			t.Fatalf("no %s tab:\n%s", watched.Title, bar)
		}
		if watchedAt > at {
			t.Errorf("%q is drawn after Settings:\n%s", watched.Title, bar)
		}
		last = watched
	}

	if gap := at - strings.Index(bar, last.Title) - len(last.Title); gap < 4 {
		t.Errorf("only %d cells between the watched tabs and Settings:\n%s", gap, bar)
	}

	press(t, program, "0")

	if got := program.Route(); got != "/settings" {
		t.Errorf("route = %q, want 0 to reach the settings", got)
	}

	var disk page.Mode
	for _, available := range modes {
		if available.Title == "Disk" {
			disk = available
			break
		}
	}

	press(t, program, keymap.Default().Keys(keymap.Global, keymap.Mode)[disk.Slot])

	if got := program.Route(); !disk.Holds(got) {
		t.Errorf("route = %q, want the disk key to reach the disk", got)
	}
}
