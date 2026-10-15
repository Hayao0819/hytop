package page_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/keymap"
	"github.com/Hayao0819/hytop/internal/ui/page"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
)

func diskGraph(t *testing.T, glyphs render.Glyphs) (*reactea.App, *store.Store) {
	t.Helper()

	memory := store.New(metric.ResolutionsFor(5 * time.Minute))
	now := time.Now().Add(-time.Second)
	for i := range 20 {
		at := now.Add(time.Duration(i-20) * time.Second)
		memory.WriteSamples([]metric.Sample{
			{Key: "diskio.total.read", Value: float64(i+1) * (1 << 20), Time: at},
			{Key: "diskio.total.write", Value: float64(i+1) * (1 << 19), Time: at},
			{Key: "diskio.sda.util", Value: float64(i + 10), Time: at},
			{Key: "diskio.nvme0n1.util", Value: float64(60 - i), Time: at},
		})
	}
	caps := render.Caps{Glyphs: glyphs, Colors: render.Mono}
	disk := page.Disk(page.Env{
		Store: memory, View: store.NewViewState(), Registry: &series.Registry{},
		Caps: caps, Theme: theme.Build(caps, nil), Keys: keymap.Default(), Span: 5 * time.Minute,
	})
	program := reactea.New(disk, reactea.WithSize(80, 24))
	program.Start()

	return program, memory
}

func TestDiskShowsSeparateThroughputAndActiveTimePlots(t *testing.T) {
	t.Parallel()

	for i, glyphs := range []render.Glyphs{render.Braille, render.Block, render.ASCII} {
		t.Run(render.GlyphNames()[i], func(t *testing.T) {
			program, _ := diskGraph(t, glyphs)
			got := testkit.Plain(program)
			for _, want := range []string{"read and write", "% active time", "100 %", "sda 29.0 %", "nvme0n1 41.0 %", "MiB/s", "Read", "Write"} {
				if !strings.Contains(got, want) {
					t.Errorf("disk page is missing %q:\n%s", want, got)
				}
			}
			if strings.Count(got, "╭") != 2 || strings.Count(got, "5m ago") != 2 {
				t.Fatalf("disk plots do not have separate frames and configured history spans:\n%s", got)
			}
		})
	}
}

func TestDiskActiveTimeFollowsNewDeviceSeries(t *testing.T) {
	t.Parallel()

	program, memory := diskGraph(t, render.Block)
	before := testkit.Plain(program)
	memory.WriteSamples([]metric.Sample{{Key: "diskio.sdb.util", Value: 95, Time: time.Now().Add(-time.Second)}})
	program.Send(struct{}{})
	after := testkit.Plain(program)
	if !strings.Contains(after, "sdb 95.0 %") || after == before {
		t.Fatalf("new disk's active-time series was not shown:\n%s", after)
	}
}

func TestDiskDoesNotInventActiveTimeWhenItIsUnavailable(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	memory.WriteSamples([]metric.Sample{
		{Key: "diskio.total.read", Value: 8192, Time: time.Now().Add(-time.Second)},
		{Key: "diskio.total.write", Value: 1024, Time: time.Now().Add(-time.Second)},
	})
	program := graphProgram(memory, nil, nil, "/graphs/disk")
	got := testkit.Plain(program)
	if !strings.Contains(got, "read and write") || !strings.Contains(got, "no data") {
		t.Fatalf("disk did not retain throughput and explain unavailable active time:\n%s", got)
	}
}

func TestDiskDisplaysMeasuredIdleTimeAsZero(t *testing.T) {
	t.Parallel()

	program, memory := diskGraph(t, render.Block)
	at := time.Now().Add(-time.Second)
	memory.WriteSamples([]metric.Sample{
		{Key: "diskio.sda.util", Value: 0, Time: at},
		{Key: "diskio.nvme0n1.util", Value: 0, Time: at},
	})
	program.Send(struct{}{})
	got := testkit.Plain(program)
	if !strings.Contains(got, "sda 0.0 %") || !strings.Contains(got, "nvme0n1 0.0 %") || strings.Contains(got, "no data") {
		t.Fatalf("measured idle time was confused with unavailable data:\n%s", got)
	}
}

func TestDiskPlotsFitSmallTerminalSizes(t *testing.T) {
	t.Parallel()

	for _, glyphs := range []render.Glyphs{render.Braille, render.Block, render.ASCII} {
		program, _ := diskGraph(t, glyphs)
		for _, size := range [][2]int{{80, 24}, {40, 16}, {28, 10}, {12, 5}, {1, 1}} {
			program.Send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			shown := testkit.Plain(program)
			lines := strings.Split(shown, "\n")
			if len(lines) > size[1] {
				t.Fatalf("disk rendered %d rows into %dx%d", len(lines), size[0], size[1])
			}
			for _, line := range lines {
				if width := lipgloss.Width(line); width > size[0] {
					t.Fatalf("disk rendered %d columns into %dx%d: %q", width, size[0], size[1], line)
				}
			}
		}
	}
}
