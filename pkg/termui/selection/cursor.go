// Package selection provides cursor and viewport arithmetic for terminal lists.
package selection

// Cursor tracks a selected row and the first visible row of a viewport.
type Cursor struct {
	Selected int
	Offset   int
}

// Reset moves the cursor and viewport to the first row.
func (c *Cursor) Reset() { c.Selected, c.Offset = 0, 0 }

// Move changes the selected row by a bounded offset.
func (c *Cursor) Move(by, total int) {
	c.Selected = min(max(c.Selected+by, 0), max(0, total-1))
}

// Top selects the first row.
func (c *Cursor) Top() { c.Reset() }

// Bottom selects the final row.
func (c *Cursor) Bottom(total int) { c.Selected = max(0, total-1) }

// Clamp repairs the cursor after rows disappear.
func (c *Cursor) Clamp(total int) {
	c.Selected = min(max(c.Selected, 0), max(0, total-1))
	c.Offset = min(max(c.Offset, 0), max(0, total-1))
}

// Reveal scrolls the viewport just far enough to include Selected.
func (c *Cursor) Reveal(visible, total int) {
	c.Clamp(total)
	visible = max(1, visible)

	switch {
	case c.Selected < c.Offset:
		c.Offset = c.Selected
	case c.Selected >= c.Offset+visible:
		c.Offset = c.Selected - visible + 1
	}

	c.Offset = min(max(c.Offset, 0), max(0, total-visible))
}
