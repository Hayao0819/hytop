package page

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
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
