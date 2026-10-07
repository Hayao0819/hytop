package chart

import (
	"math"
	"strings"
)

var eighths = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}

const (
	full  = '█'
	empty = '░'
)

// Gauge returns the filled and empty portions of a horizontal bar separately.
func Gauge(fraction float64, width int, glyphs Glyphs) (filled, rest string) {
	if width <= 0 {
		return "", ""
	}

	if math.IsNaN(fraction) || fraction < 0 {
		fraction = 0
	}

	fraction = min(fraction, 1)

	if glyphs == ASCII {
		on := int(math.Round(fraction * float64(width)))

		return strings.Repeat("#", on), strings.Repeat(".", width-on)
	}

	var (
		steps = int(math.Round(fraction * float64(width) * 8))
		whole = steps / 8
		part  = steps % 8
	)

	if whole >= width {
		return strings.Repeat(string(full), width), ""
	}

	filled = strings.Repeat(string(full), whole)
	if part > 0 {
		filled += string(eighths[part])
	}

	return filled, strings.Repeat(string(empty), width-whole-min(part, 1))
}
