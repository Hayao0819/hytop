package summary_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/summary"
)

func TestNoDrivesAreNotReportedAsPassingSMART(t *testing.T) {
	t.Parallel()

	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	widget := summary.NewDisk(store.New(nil), theme.Build(caps, nil), caps, nil, nil)
	program := reactea.New(widget, reactea.WithSize(60, summary.Rows))
	_ = program.Init()

	got := testkit.Plain(program)
	if !strings.Contains(got, "0 drives") {
		t.Fatalf("empty disk summary does not report its drive count: %q", got)
	}
	if strings.Contains(got, "passing") {
		t.Fatalf("empty disk summary claims SMART passed: %q", got)
	}
}
