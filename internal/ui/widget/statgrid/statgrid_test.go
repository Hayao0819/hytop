package statgrid_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/widget/statgrid"
)

func TestLongCellsDoNotPushOutTheirNeighbours(t *testing.T) {
	t.Parallel()

	memory := store.New(nil)
	memory.WriteFacts(map[string]string{"long": "非常に長いメモリモジュール型番ABCDEFGHIJK"})
	widget := statgrid.New(memory,
		statgrid.Stat{Label: "長いラベルがセルの幅を超える", Fact: "long"},
		statgrid.Stat{Label: "Second", Text: "visible"},
	)
	program := reactea.New(widget, reactea.WithSize(36, 2))
	_ = program.Init()
	got := testkit.Plain(program)

	if !strings.Contains(got, "Second") || !strings.Contains(got, "visible") {
		t.Fatalf("the second cell was pushed out:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if width := lipgloss.Width(line); width != 36 {
			t.Errorf("line width = %d, want 36: %q", width, line)
		}
	}
}
