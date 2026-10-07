// Package safe makes text from outside hytop safe to put on a terminal.
package safe

import (
	"strings"
	"unicode/utf8"
)

// Text replaces terminal controls, bidi controls, and invalid UTF-8 with safe
// visible text; tabs and line breaks become spaces.
func Text(s string) string {
	if !suspect(s) {
		return s
	}

	var out strings.Builder

	out.Grow(len(s))

	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		switch {
		case r == utf8.RuneError && size == 1:
			out.WriteByte('?')
		case r == '\t', r == '\n', r == '\r':
			out.WriteByte(' ')
		case control(r) || bidiControl(r):
			out.WriteByte('?')
		default:
			out.WriteRune(r)
		}
	}

	return out.String()
}

func suspect(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if control(r) || bidiControl(r) {
			return true
		}
	}

	return false
}

// control covers C0, DEL, and C1 controls, including U+009B CSI.
func control(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

func bidiControl(r rune) bool {
	return r == 0x061c || r == 0x200e || r == 0x200f ||
		(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x206f)
}
