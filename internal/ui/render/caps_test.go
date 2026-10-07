package render

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestDetectGlyphs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		environ []string
		want    Glyphs
	}{
		{"Windows Terminal", "windows", []string{"WT_SESSION=1"}, Braille},
		{"Windows console", "windows", nil, Block},
		{"Linux console", "linux", []string{"TERM=linux", "LANG=en_US.UTF-8"}, Block},
		{"UTF-8 xterm", "linux", []string{"TERM=xterm-256color", "LANG=en_US.UTF-8"}, Braille},
		{"UTF-8 spelling", "linux", []string{"TERM=xterm", "LANG=en_US.utf8"}, Braille},
		{"non-UTF-8 xterm", "linux", []string{"TERM=xterm", "LANG=C"}, ASCII},
		{"dumb terminal", "windows", []string{"TERM=dumb", "WT_SESSION=1"}, ASCII},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := detectGlyphs(test.goos, test.environ); got != test.want {
				t.Errorf("detectGlyphs() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestForegroundHonoursMonoMode(t *testing.T) {
	t.Parallel()

	got := Foreground(Caps{Colors: Mono}, lipgloss.Color("1")).Render("value")
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("mono foreground emitted styling: %q", got)
	}
}

func TestDetectColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile colorprofile.Profile
		want    Colors
	}{
		{colorprofile.TrueColor, TrueColor},
		{colorprofile.ANSI256, Ansi256},
		{colorprofile.ANSI, Ansi16},
		{colorprofile.ASCII, Mono},
		{colorprofile.NoTTY, Mono},
	}

	for _, test := range tests {
		if got := detectColors(test.profile); got != test.want {
			t.Errorf("detectColors(%v) = %v, want %v", test.profile, got, test.want)
		}
	}
}

func TestDetectUsesTerminalEnvironment(t *testing.T) {
	t.Parallel()

	got := Detect([]string{"TERM=xterm-256color", "LANG=en_US.UTF-8"})
	want := Caps{Glyphs: Braille, Colors: Ansi256}
	if got != want {
		t.Errorf("Detect() = %+v, want %+v", got, want)
	}
}
