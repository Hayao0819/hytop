package selection

import "testing"

func TestCursorRevealAndClamp(t *testing.T) {
	t.Parallel()

	c := Cursor{}
	c.Move(7, 10)
	c.Reveal(3, 10)
	if c.Selected != 7 || c.Offset != 5 {
		t.Fatalf("after reveal: %+v", c)
	}

	c.Clamp(2)
	if c.Selected != 1 || c.Offset != 1 {
		t.Fatalf("after rows shrink: %+v", c)
	}

	c.Top()
	if c != (Cursor{}) {
		t.Fatalf("after top: %+v", c)
	}
}

func TestCursorRevealWithALargeViewport(t *testing.T) {
	t.Parallel()

	c := Cursor{Selected: 5, Offset: 2}
	c.Reveal(int(^uint(0)>>1), 10)
	if c.Selected != 5 || c.Offset != 0 {
		t.Fatalf("large viewport: %+v, want selected=5, offset=0", c)
	}
}
