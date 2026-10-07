// Package render describes terminal display capabilities.
package render

import (
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/colorprofile"

	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

type Glyphs = chart.Glyphs

const (
	Braille = chart.Braille
	Block   = chart.Block
	ASCII   = chart.ASCII
)

type Colors int

const (
	TrueColor Colors = iota
	Ansi256
	Ansi16
	Mono
)

type Caps struct {
	Glyphs Glyphs
	Colors Colors
}

var (
	glyphNames = map[Glyphs]string{Braille: "braille", Block: "block", ASCII: "ascii"}
	colorNames = map[Colors]string{TrueColor: "truecolor", Ansi256: "256", Ansi16: "16", Mono: "mono"}
)

func (c Colors) String() string { return colorNames[c] }

func ParseGlyphs(name string) (Glyphs, bool) {
	for glyphs, known := range glyphNames {
		if known == name {
			return glyphs, true
		}
	}

	return Braille, false
}

func ParseColors(name string) (Colors, bool) {
	for colors, known := range colorNames {
		if known == name {
			return colors, true
		}
	}

	return TrueColor, false
}

// GlyphNames and ColorNames are ordered finest first, which is the order the
// help and the settings screen want to offer them in.
func GlyphNames() []string { return []string{"braille", "block", "ascii"} }

func ColorNames() []string { return []string{"truecolor", "256", "16", "mono"} }

func Detect(environ []string) Caps {
	if environ == nil {
		environ = os.Environ()
	}

	return Caps{
		Glyphs: detectGlyphs(runtime.GOOS, environ),
		Colors: detectColors(colorprofile.Env(environ)),
	}
}

func detectGlyphs(goos string, environ []string) Glyphs {
	term := envValue(environ, "TERM")
	if term == "dumb" {
		return ASCII
	}
	if term == "linux" {
		return Block
	}
	if goos == "windows" && term == "" {
		if envValue(environ, "WT_SESSION") != "" {
			return Braille
		}

		return Block
	}
	if term == "" || !utf8Locale(environ) {
		return ASCII
	}

	return Braille
}

func detectColors(profile colorprofile.Profile) Colors {
	switch profile {
	case colorprofile.TrueColor:
		return TrueColor
	case colorprofile.ANSI256:
		return Ansi256
	case colorprofile.ANSI:
		return Ansi16
	default:
		return Mono
	}
}

func utf8Locale(environ []string) bool {
	locale := strings.ToLower(
		envValue(environ, "LANG") + envValue(environ, "LC_ALL") + envValue(environ, "LC_CTYPE"),
	)

	return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
}

func envValue(environ []string, name string) string {
	for i := len(environ) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(environ[i], "=")
		if ok && strings.EqualFold(key, name) {
			return value
		}
	}

	return ""
}
