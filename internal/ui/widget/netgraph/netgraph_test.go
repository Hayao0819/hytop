package netgraph_test

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/widget/netgraph"
)

func networkWidget() *netgraph.Widget {
	memory := store.New(metric.DefaultResolutions())
	now := time.Now()
	memory.WriteSamples([]metric.Sample{
		{Key: "net.eth0.rx", Value: 2048, Time: now.Add(-time.Second)},
		{Key: "net.eth0.tx", Value: 1024, Time: now.Add(-time.Second)},
	})

	return netgraph.New(
		memory,
		render.Caps{Glyphs: render.ASCII, Colors: render.Mono},
		"eth0",
		lipgloss.Color("1"),
		lipgloss.Color("2"),
	)
}

func TestNetworkGraphFitsOneRow(t *testing.T) {
	t.Parallel()

	program := reactea.New(networkWidget(), reactea.WithSize(20, 1))
	_ = program.Init()

	if width, height := lipgloss.Size(testkit.Plain(program)); width != 20 || height != 1 {
		t.Fatalf("one-row graph size = %dx%d, want 20x1: %q", width, height, testkit.Plain(program))
	}
}

func TestNetworkGraphHonoursASCIIMonoFallback(t *testing.T) {
	t.Parallel()

	program := reactea.New(networkWidget(), reactea.WithSize(20, 5))
	_ = program.Init()

	plain := testkit.Plain(program)
	if width, height := lipgloss.Size(plain); width != 20 || height != 5 {
		t.Fatalf("graph size = %dx%d, want 20x5:\n%s", width, height, plain)
	}
	if strings.ContainsAny(plain, "▁▂▃▄▅▆▇█⠀⣿") {
		t.Fatalf("ASCII graph emitted a Unicode graph rune:\n%s", plain)
	}
	if raw := program.View().Content; strings.Contains(raw, "\x1b[38;") || strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("mono graph emitted a colour sequence: %q", raw)
	}
}
