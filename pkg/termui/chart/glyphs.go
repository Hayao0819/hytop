package chart

// Glyphs selects the character set used to draw a chart.
type Glyphs int

const (
	// Braille draws at two columns and four rows of dots per terminal cell.
	Braille Glyphs = iota
	// Block draws with partial block characters.
	Block
	// ASCII draws with portable single-width characters.
	ASCII
)

var glyphNames = [...]string{"braille", "block", "ascii"}

// String returns the stable configuration name for the glyph mode.
func (g Glyphs) String() string {
	if g < Braille || int(g) >= len(glyphNames) {
		return "unknown"
	}

	return glyphNames[g]
}
