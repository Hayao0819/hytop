package render_test

import (
	"fmt"
	"testing"

	"github.com/Hayao0819/hytop/internal/ui/render"
)

func TestTableClipsAndPadsAViewport(t *testing.T) {
	t.Parallel()

	got := render.Table(4, "head", "", 5, 2, func(index int) string { return fmt.Sprint(index) })
	if want := "head\n2\n3\n4"; got != want {
		t.Fatalf("Table() = %q, want %q", got, want)
	}

	got = render.Table(4, "head", "empty", 0, 0, nil)
	if want := "head\nempty\n\n"; got != want {
		t.Fatalf("empty Table() = %q, want %q", got, want)
	}

	got = render.Table(2, "head", "", 1, -2, func(index int) string { return fmt.Sprint(index) })
	if want := "head\n0"; got != want {
		t.Fatalf("negative offset Table() = %q, want %q", got, want)
	}
}

func TestTableFitsAHeaderOnlyViewport(t *testing.T) {
	t.Parallel()

	for _, height := range []int{-1, 0, 1} {
		for _, count := range []int{0, 1} {
			got := render.Table(height, "head", "empty", count, 0, func(int) string {
				t.Fatal("rendered a row without room below the header")

				return ""
			})
			want := ""
			if height == 1 {
				want = "head"
			}
			if got != want {
				t.Errorf("Table(height=%d, count=%d) = %q, want %q", height, count, got, want)
			}
		}
	}
}
