package page

import (
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func graphStyleEnv() Env {
	memory := store.New(metric.DefaultResolutions())
	now := time.Now()
	for _, key := range []series.Key{
		"cpu.total.usage", "cpu.core.0.usage", "mem.usage", "diskio.total.read", "diskio.sda.util",
		"thermal.max.temp", "gpu.0.util", "battery.0.capacity", "cpu.package.power", "net.eth0.rx", "net.eth0.tx",
	} {
		value := 75.0
		if strings.HasPrefix(string(key), "net.") {
			value = 2048
		}
		for _, age := range []time.Duration{59 * time.Second, time.Second} {
			memory.WriteSamples([]metric.Sample{{Key: key, Value: value, Time: now.Add(-age)}})
		}
	}
	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	return Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Caps: caps, Theme: theme.Build(caps, nil), Keys: keymap.Default(), Span: time.Minute,
		Interfaces: func() []string { return []string{"eth0"} },
	}
}

func TestBuiltInGraphsUseTheConfiguredStyle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		build func(Env) reactea.Component
		keys  []string
	}{
		{"CPU", CPU, nil},
		{"cores", CPU, []string{"c"}},
		{"sidebar", func(env Env) reactea.Component { return NewGraphs(env).rail }, nil},
		{"memory", Memory, nil},
		{"disk", Disk, nil},
		{"sensors", Sensors, nil},
		{"GPU", func(env Env) reactea.Component { return GPU(env, 0, "GPU") }, nil},
		{"battery", func(env Env) reactea.Component { return Battery(env, 0, "Battery") }, nil},
		{"power", func(env Env) reactea.Component { return Power(env, []series.Key{"cpu.package.power"}) }, nil},
		{"network", Network, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := graphStyleEnv()
			var cells [2]int
			for i, style := range []string{conf.GraphFilled, conf.GraphLine} {
				env.GraphStyle = style
				program := reactea.New(test.build(env), reactea.WithSize(80, 40))
				program.Start()
				testkit.SendKeys(program, test.keys...)
				cells[i] = strings.Count(testkit.Plain(program), "#")
			}
			if cells[0] <= cells[1] {
				t.Fatalf("filled plot has %d cells versus %d line cells", cells[0], cells[1])
			}
		})
	}
}

func TestCustomGraphFillOverridesTheGlobalStyle(t *testing.T) {
	t.Parallel()

	for _, style := range []string{conf.GraphFilled, conf.GraphLine} {
		env := graphStyleEnv()
		env.GraphStyle = style
		var cells [3]int
		for i, fill := range []*bool{nil, new(true), new(false)} {
			component, err := dashboardGraph(env, conf.Pane{
				Widget: conf.WidgetBraille, Series: conf.StringList{"cpu.total.usage"}, Fill: fill,
			})
			if err != nil {
				t.Fatal(err)
			}
			program := reactea.New(component, reactea.WithSize(40, 12))
			program.Start()
			cells[i] = strings.Count(testkit.Plain(program), "#")
		}
		if cells[1] <= cells[2] {
			t.Fatalf("%s: explicit filled graph has %d cells versus %d line cells", style, cells[1], cells[2])
		}
		inherited := 1
		if style == conf.GraphLine {
			inherited = 2
		}
		if cells[0] != cells[inherited] {
			t.Fatalf("%s: inherited graph has %d cells, want %d", style, cells[0], cells[inherited])
		}
	}
}
