package gauge_test

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	gaugewidget "github.com/Hayao0819/hytop/internal/ui/widget/gauge"
)

func TestGaugeShowsValueAndThresholdWithoutRelyingOnColour(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	memory.WriteSamples([]metric.Sample{{Key: "cpu.total.usage", Value: 75, Time: time.Now()}})
	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	widget := gaugewidget.New(memory, theme.Build(caps, nil), caps, "cpu.total.usage")
	widget.Unit = series.Percent
	widget.Threshold = []float64{70, 90}

	program := reactea.New(widget, reactea.WithSize(20, 1))
	_ = program.Init()
	got := testkit.Plain(program)

	if !strings.Contains(got, "▲ 75 %") || !strings.ContainsAny(got, "#.") {
		t.Errorf("warning gauge = %q", got)
	}
	if width := lipgloss.Width(got); width != 20 {
		t.Errorf("gauge width = %d, want 20", width)
	}

	memory.WriteSamples([]metric.Sample{{Key: "cpu.total.usage", Value: 95, Time: time.Now()}})
	if got := testkit.Plain(program); !strings.Contains(got, "◆ 95 %") {
		t.Errorf("critical gauge = %q", got)
	}
}
