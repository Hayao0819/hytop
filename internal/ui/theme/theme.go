// Package theme maps semantic tokens to terminal styles.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/Hayao0819/hytop/internal/ui/render"
)

type Token int

const (
	Text Token = iota
	Label
	Dim
	Heading
	Selected
	Border
	BorderActive
	TableHeader
	Tab
	TabActive

	CPU
	CPUPressure
	Memory
	Swap
	DiskRead
	DiskWrite
	NetRx
	NetTx

	Nice
	Warn
	Critical
	Own
	Root
	Kernel
	Tree
)

// tokenNames preserves the settings-screen order.
var tokenNames = []struct {
	token Token
	name  string
}{
	{Text, "text"},
	{Label, "label"},
	{Dim, "dim"},
	{Heading, "heading"},
	{Selected, "selected"},
	{Border, "border"},
	{BorderActive, "border-active"},
	{TableHeader, "table-header"},
	{Tab, "tab"},
	{TabActive, "tab-active"},
	{CPU, "cpu"},
	{CPUPressure, "cpu-pressure"},
	{Memory, "memory"},
	{Swap, "swap"},
	{DiskRead, "disk-read"},
	{DiskWrite, "disk-write"},
	{NetRx, "net-rx"},
	{NetTx, "net-tx"},
	{Nice, "nice"},
	{Warn, "warn"},
	{Critical, "critical"},
	{Own, "own"},
	{Root, "root"},
	{Kernel, "kernel"},
	{Tree, "tree"},
}

func (t Token) String() string {
	for _, entry := range tokenNames {
		if entry.token == t {
			return entry.name
		}
	}

	return "text"
}

func ParseToken(name string) (Token, bool) {
	for _, entry := range tokenNames {
		if entry.name == name {
			return entry.token, true
		}
	}

	return Text, false
}

// TokenNames lists every token a palette may name.
func TokenNames() []string {
	names := make([]string, 0, len(tokenNames))
	for _, entry := range tokenNames {
		names = append(names, entry.name)
	}

	return names
}

// Theme contains styles resolved for one palette and terminal profile.
type Theme struct {
	caps   render.Caps
	styles map[Token]lipgloss.Style
	colour map[Token]color.Color
}

// Options contains terminal properties that are not part of the colour
// palette. Dark is nil when the terminal did not report its background.
type Options struct {
	HighContrast bool
	Muted        bool
	Dark         *bool
}

func (t *Theme) Style(token Token) lipgloss.Style {
	if style, ok := t.styles[token]; ok {
		return style
	}

	return lipgloss.NewStyle()
}

func (t *Theme) Colour(token Token) color.Color {
	if t.caps.Colors == render.Mono {
		return lipgloss.NoColor{}
	}

	if c, ok := t.colour[token]; ok {
		return c
	}

	return lipgloss.Color("7")
}

// Threshold picks a token from a reading and its warning and danger points.
func (t *Theme) Threshold(value, warn, critical float64) Token {
	switch {
	case value >= critical:
		return Critical
	case value >= warn:
		return Warn
	case value <= 0:
		return Dim
	default:
		return Text
	}
}

// Build resolves a palette while retaining each token's text attributes.
func Build(caps render.Caps, palette map[Token]color.Color) *Theme {
	return BuildWith(caps, palette, Options{})
}

// BuildWith resolves a palette for the terminal and accessibility settings.
func BuildWith(caps render.Caps, palette map[Token]color.Color, options Options) *Theme {
	colours := map[Token]color.Color{
		Text:         lipgloss.NoColor{},
		Label:        lipgloss.NoColor{},
		Dim:          lipgloss.Color("8"),
		Heading:      lipgloss.Color("14"),
		Border:       lipgloss.Color("8"),
		BorderActive: lipgloss.Color("6"),
		TableHeader:  lipgloss.Color("14"),
		Tab:          lipgloss.Color("7"),
		TabActive:    lipgloss.Color("6"),

		CPU:         lipgloss.Color("4"),
		CPUPressure: lipgloss.Color("1"),
		Memory:      lipgloss.Color("5"),
		Swap:        lipgloss.Color("3"),
		DiskRead:    lipgloss.Color("2"),
		DiskWrite:   lipgloss.Color("1"),
		NetRx:       lipgloss.Color("6"),
		NetTx:       lipgloss.Color("3"),

		Nice:     lipgloss.Color("6"),
		Warn:     lipgloss.Color("3"),
		Critical: lipgloss.Color("1"),
		Own:      lipgloss.Color("2"),
		Root:     lipgloss.Color("1"),
		Kernel:   lipgloss.Color("8"),
		Tree:     lipgloss.Color("8"),
	}
	if !options.Muted {
		for _, token := range []Token{Dim, Kernel, Tree, Tab} {
			colours[token] = lipgloss.NoColor{}
		}
	}
	if options.Dark != nil && !*options.Dark {
		colours[Heading] = lipgloss.Color("4")
		colours[BorderActive] = lipgloss.Color("4")
		colours[TableHeader] = lipgloss.Color("4")
		colours[TabActive] = lipgloss.Color("4")
	}

	for token, c := range palette {
		colours[token] = c
	}

	t := &Theme{caps: caps, colour: colours, styles: map[Token]lipgloss.Style{}}

	for token, c := range colours {
		style := lipgloss.NewStyle()

		if caps.Colors != render.Mono {
			style = style.Foreground(c)
		}

		t.styles[token] = style
	}

	if options.HighContrast {
		t.styles[Label] = t.styles[Label].Bold(true)
		t.styles[Border] = lipgloss.NewStyle()
	}
	t.styles[Heading] = t.styles[Heading].Bold(true)
	t.styles[TabActive] = t.styles[TabActive].Bold(true)
	t.styles[Critical] = t.styles[Critical].Bold(true)

	t.styles[TableHeader] = lipgloss.NewStyle().
		Foreground(colours[TableHeader]).
		Background(colours[Border]).
		Bold(true)

	t.styles[Selected] = lipgloss.NewStyle().Reverse(true)

	if caps.Colors == render.Mono {
		t.styles[TableHeader] = lipgloss.NewStyle().Bold(true).Reverse(true)
		t.styles[TabActive] = lipgloss.NewStyle().Bold(true)
	}

	return t
}
