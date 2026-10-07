package devicelist_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/render"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/devicelist"
)

func rail(t *testing.T, route string) *reactea.App {
	t.Helper()

	caps := render.Caps{Glyphs: render.ASCII, Colors: render.Mono}
	entries := make([]devicelist.Entry, 7)
	for i := range entries {
		entries[i] = devicelist.Entry{Title: fmt.Sprintf("Device %d", i), Route: fmt.Sprintf("/%d", i)}
	}

	program := reactea.New(
		devicelist.New(store.New(nil), caps, theme.Build(caps, nil), entries...),
		reactea.WithSize(20, 20), reactea.WithRoute(route),
	)
	_ = program.Init()

	return program
}

func TestRailShowsHowManyDevicesContinueBelow(t *testing.T) {
	t.Parallel()

	program := rail(t, "/0")
	got := testkit.Plain(program)

	if !strings.Contains(got, "v 3 more") {
		t.Fatalf("rail has no lower overflow indicator:\n%s", got)
	}
	if strings.Contains(got, "Device 4") {
		t.Fatalf("rail drew an entry through its overflow indicator:\n%s", got)
	}
	if strings.ContainsAny(got, "↑↓") {
		t.Fatalf("ASCII rail used Unicode arrows:\n%s", got)
	}
}

func TestRailShowsHiddenDevicesAboveAndDoesNotClickTheIndicator(t *testing.T) {
	t.Parallel()

	program := rail(t, "/6")
	got := testkit.Plain(program)
	if !strings.Contains(got, "^ 3 more") || !strings.Contains(got, "Device 6") {
		t.Fatalf("selected device was not kept visible with an upper indicator:\n%s", got)
	}

	program.Send(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	if got := program.Route(); got != "/6" {
		t.Fatalf("clicking the overflow indicator changed the route to %q", got)
	}

	program.Send(tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft})
	if got := program.Route(); got != "/3" {
		t.Fatalf("clicking the first visible entry changed the route to %q", got)
	}
}
