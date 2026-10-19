package graph_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/widget/graph"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

func TestAPatternFollowsNewLiveSeries(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	now := time.Now()
	memory.WriteSamples([]metric.Sample{{Key: "cpu.core.0.usage", Value: 10, Time: now}})

	widget := graph.New(memory, render.Caps{Glyphs: render.Block, Colors: render.Mono},
		graph.Track{Pattern: "cpu.core.*.usage"},
	)
	widget.Range = chart.Range{Max: 100}
	program := reactea.New(widget, reactea.WithSize(8, 3))
	_ = program.Init()

	before := testkit.Plain(program)
	memory.WriteSamples([]metric.Sample{{Key: "cpu.core.1.usage", Value: 100, Time: now}})
	after := testkit.Plain(program)

	if before == after {
		t.Fatalf("a newly matching series did not change the graph:\n%s", after)
	}
}

func TestGraphExplainsMissingData(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	widget := graph.New(memory, render.Caps{Glyphs: render.Block, Colors: render.Mono},
		graph.Track{Key: "cpu.total.usage"},
	)
	program := reactea.New(widget, reactea.WithSize(12, 3))
	_ = program.Init()

	if got := testkit.Plain(program); !strings.Contains(got, "no data") {
		t.Fatalf("empty graph = %q, want a no-data explanation", got)
	}
}

func TestGraphTreatsInvalidReadingsAsMissing(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	memory.WriteSamples([]metric.Sample{
		{Key: "cpu.total.usage", Value: math.NaN(), Time: time.Now()},
		{Key: "cpu.total.usage", Value: math.Inf(1), Time: time.Now().Add(time.Millisecond)},
	})
	widget := graph.New(memory, render.Caps{Glyphs: render.Block, Colors: render.Mono},
		graph.Track{Key: "cpu.total.usage"},
	)
	program := reactea.New(widget, reactea.WithSize(12, 3))
	_ = program.Init()

	if got := testkit.Plain(program); !strings.Contains(got, "no data") {
		t.Fatalf("invalid-only graph = %q, want a no-data explanation", got)
	}
}

func TestGraphCanRenderAsOneLine(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	memory.WriteSamples([]metric.Sample{{Key: "cpu.total.usage", Value: 50, Time: time.Now()}})
	widget := graph.New(memory, render.Caps{Glyphs: render.Block, Colors: render.Mono},
		graph.Track{Key: "cpu.total.usage"},
	)
	widget.MaxHeight = 1
	program := reactea.New(widget, reactea.WithSize(12, 4))
	_ = program.Init()

	if got := testkit.Plain(program); strings.Contains(got, "\n") {
		t.Fatalf("one-line graph contains a newline: %q", got)
	}
}

func TestAutoRangeLabelsItsResolvedMinimum(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	memory.WriteSamples([]metric.Sample{{Key: "custom.temperature", Value: -10, Time: time.Now()}})
	widget := graph.New(memory, render.Caps{Glyphs: render.Block, Colors: render.Mono},
		graph.Track{Key: "custom.temperature"},
	)
	widget.Range = chart.Range{}
	widget.Unit = series.None
	program := reactea.New(widget, reactea.WithSize(20, 4))
	_ = program.Init()

	lines := strings.Split(testkit.Plain(program), "\n")
	if !strings.HasSuffix(lines[len(lines)-1], "-10") {
		t.Fatalf("bottom axis = %q, want resolved minimum -10", lines[len(lines)-1])
	}
}

func TestGraphConnectsReadingsAfterResampling(t *testing.T) {
	t.Parallel()

	now := time.Now()
	memory := store.New(metric.DefaultResolutions())
	for _, sample := range []metric.Sample{
		{Key: "diskio.total.read", Value: 0, Time: now.Add(-50 * time.Second)},
		{Key: "diskio.total.read", Value: 100, Time: now.Add(-30 * time.Second)},
		{Key: "diskio.total.read", Value: 0, Time: now.Add(-10 * time.Second)},
	} {
		memory.WriteSamples([]metric.Sample{sample})
	}
	for _, glyphs := range []render.Glyphs{render.Braille, render.Block, render.ASCII} {
		for _, axes := range []bool{false, true} {
			widget := graph.New(memory, render.Caps{Glyphs: glyphs, Colors: render.Mono},
				graph.Track{Key: "diskio.total.read"},
			)
			widget.Axis, widget.Fill, widget.Label = axes, false, "read and write"
			program := reactea.New(widget, reactea.WithSize(40, 12))
			_ = program.Init()
			shown := testkit.Plain(program)
			rows := strings.Split(shown, "\n")
			if len(rows) != 12 {
				t.Fatalf("%s axes=%t: got %d rows, want 12", glyphs, axes, len(rows))
			}
			if axes {
				if !strings.Contains(rows[0], widget.Label) || !strings.HasSuffix(rows[0], "100 %") ||
					!strings.Contains(rows[11], "1m ago") || !strings.HasSuffix(rows[11], "0 %") {
					t.Fatalf("%s: curve obscured the axis labels:\n%s", glyphs, shown)
				}
				rows = rows[1:11]
			}
			for y, row := range rows {
				if strings.TrimSpace(row) == "" {
					t.Fatalf("%s axes=%t: row %d has a gap in the stroke:\n%s", glyphs, axes, y, shown)
				}
			}
		}
	}
}
