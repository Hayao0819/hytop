package coregrid_test

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
	"github.com/Hayao0819/hytop/internal/ui/widget/coregrid"
)

func TestGridFitsItsBoxWithoutColourInMonoMode(t *testing.T) {
	t.Parallel()

	memory := store.New(metric.DefaultResolutions())
	now := time.Now()
	memory.WriteSamples([]metric.Sample{
		{Key: "cpu.core.0.usage", Value: 25, Time: now},
		{Key: "cpu.core.1.usage", Value: 75, Time: now},
	})
	keys := []series.Key{"cpu.core.0.usage", "cpu.core.1.usage"}
	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	widget := coregrid.New(memory, caps, lipgloss.Color("1"), func() []series.Key { return keys })
	program := reactea.New(widget, reactea.WithSize(24, 10))
	_ = program.Init()

	plain := testkit.Plain(program)
	if width, height := lipgloss.Size(plain); width != 24 || height != 10 {
		t.Fatalf("grid size = %dx%d, want 24x10:\n%s", width, height, plain)
	}
	if raw := program.View().Content; strings.Contains(raw, "\x1b[38;") || strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("mono grid emitted a colour sequence: %q", raw)
	}
}
