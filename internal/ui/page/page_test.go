package page

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func TestPaneRendersEveryBorder(t *testing.T) {
	lines := strings.Split(pane.Width(8).Height(3).Render("x"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "╭") ||
		!strings.HasPrefix(lines[1], "│") || !strings.HasPrefix(lines[2], "╰") {
		t.Fatalf("pane has no left border:\n%s", strings.Join(lines, "\n"))
	}
}

func TestDesktopPowerPageIsReplacedByBatteryPages(t *testing.T) {
	t.Parallel()

	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	memory := store.New(nil)
	memory.WriteSamples([]metric.Sample{
		{Key: "cpu.package.power", Value: 35, Time: time.Now()},
		{Key: "power.corsairpsu_power1.watts", Value: 120, Time: time.Now()},
	})
	env := Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Theme: theme.Build(caps, nil), Keys: keymap.Default(), Caps: caps, Span: time.Minute,
	}

	desktop := NewGraphs(env)
	if !hasGraphRoute(desktop.specs, "/graphs/power") {
		t.Fatalf("desktop graph routes = %v, want a power page", graphRoutes(desktop.specs))
	}

	env.Batteries = func() []int { return []int{0} }
	laptop := NewGraphs(env)
	if hasGraphRoute(laptop.specs, "/graphs/power") {
		t.Fatalf("battery graph routes = %v, should not duplicate the power page", graphRoutes(laptop.specs))
	}
	if !hasGraphRoute(laptop.specs, "/graphs/battery/0") {
		t.Fatalf("battery graph routes = %v, want a battery page", graphRoutes(laptop.specs))
	}
}

func TestPowerSeriesDeduplicatesOnlyKnownSensorAliases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		gpu    bool
		source string
		want   []series.Key
	}{
		{"alias", true, "/sys/devices/gpu0/hwmon/power1", []series.Key{"gpu.0.power"}},
		{"different sensor", true, "/sys/devices/gpu1/hwmon/power1", []series.Key{"gpu.0.power", "power.amdgpu_power1.watts"}},
		{"unknown source", true, "", []series.Key{"gpu.0.power", "power.amdgpu_power1.watts"}},
		{"hwmon fallback", false, "/sys/devices/gpu0/hwmon/power1", []series.Key{"power.amdgpu_power1.watts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			memory := store.New(nil)
			memory.WriteSamples([]metric.Sample{{Key: "power.amdgpu_power1.watts", Value: 30, Time: time.Now()}})
			if tc.gpu {
				memory.WriteSamples([]metric.Sample{{Key: "gpu.0.power", Value: 30, Time: time.Now()}})
			}
			memory.WriteFacts(map[string]string{
				"gpu.0.power.source":               "/sys/devices/gpu0/hwmon/power1",
				"power.amdgpu_power1.watts.source": tc.source,
			})
			if got := powerSeries(memory); !slices.Equal(got, tc.want) {
				t.Fatalf("power series = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReturningFromAProcessFilterResetsTheViewport(t *testing.T) {
	t.Parallel()

	view := store.NewViewState()
	expr, err := filter.Compile("cpu > 1")
	if err != nil {
		t.Fatal(err)
	}
	view.PushFilter("cpu > 1", expr)
	ids := []procmodel.Identity{{PID: 1}, {PID: 2}, {PID: 3}}
	view.SelectProcess(2, ids[2])
	view.SetOffset(2)
	p := &Processes{view: view}
	p.back()

	if src, _ := view.Filter(); src != "" {
		t.Fatalf("previous filter = %q, want empty", src)
	}
	if view.Offset() != 0 || view.ReconcileProcesses(ids) != 0 {
		t.Fatalf("previous filter retained viewport: selected=%d offset=%d", view.Selected(), view.Offset())
	}
}

func hasGraphRoute(specs []GraphSpec, route string) bool {
	for _, spec := range specs {
		if spec.Route == route {
			return true
		}
	}

	return false
}

func graphRoutes(specs []GraphSpec) []string {
	routes := make([]string, len(specs))
	for i, spec := range specs {
		routes[i] = spec.Route
	}

	return routes
}

func TestFilesystemOptionsAreTruncatedByCells(t *testing.T) {
	t.Parallel()

	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	page := newFilesystems(Env{
		Store: store.New(nil), Theme: theme.Build(caps, nil), Keys: keymap.Default(),
	}, nil, nil)
	lines := page.detail(22, diskmodel.Mount{Opts: "圧縮方式=zstd,高速モード"})

	if !utf8.ValidString(lines[2]) {
		t.Fatalf("truncation split a UTF-8 sequence: %q", lines[2])
	}
	if !strings.Contains(lines[2], "…") {
		t.Fatalf("truncated options have no ellipsis: %q", lines[2])
	}
}
