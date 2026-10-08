package selection_test

import (
	"fmt"
	"testing"

	"github.com/Hayao0819/hytop/pkg/termui/selection"
)

func TestWindow(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name                   string
		offset, visible, total int
		start, end             int
	}{
		{"middle", 3, 4, 10, 3, 7},
		{"past end", 20, 4, 10, 6, 10},
		{"negative offset", -3, 4, 10, 0, 4},
		{"short list", 3, 10, 4, 0, 4},
		{"empty list", 3, 4, 0, 0, 0},
		{"negative total", 3, 4, -1, 0, 0},
		{"zero height", 3, 0, 10, 3, 3},
		{"zero height past end", 20, 0, 10, 10, 10},
		{"negative height", 3, -1, 10, 3, 3},
		{"large offset", maxInt, 4, 10, 6, 10},
		{"large height", 3, maxInt, 10, 0, 10},
		{"large total", maxInt, 4, maxInt, maxInt - 4, maxInt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start, end := selection.Window(tc.offset, tc.visible, tc.total)
			if start != tc.start || end != tc.end {
				t.Errorf("Window(%d, %d, %d) = [%d, %d), want [%d, %d)",
					tc.offset, tc.visible, tc.total, start, end, tc.start, tc.end)
			}
		})
	}
}

func FuzzWindow(f *testing.F) {
	f.Add(3, 4, 10)
	f.Add(-1, -1, -1)
	f.Add(int(^uint(0)>>1), 4, 10)
	f.Fuzz(func(t *testing.T, offset, visible, total int) {
		start, end := selection.Window(offset, visible, total)
		total = max(0, total)
		visible = max(0, visible)
		if start < 0 || end < start || end > total || end-start != min(visible, total) {
			t.Fatalf("invalid window [%d, %d) for offset=%d, visible=%d, total=%d",
				start, end, offset, visible, total)
		}
	})
}

func ExampleWindow() {
	rows := []string{"a", "b", "c", "d", "e"}
	start, end := selection.Window(4, 3, len(rows))
	fmt.Println(rows[start:end])
	// Output: [c d e]
}
