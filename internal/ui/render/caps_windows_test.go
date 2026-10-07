//go:build windows

package render

import "testing"

func TestDetectWindowsTerminal(t *testing.T) {
	t.Parallel()

	want := Caps{Glyphs: Braille, Colors: TrueColor}
	if got := Detect([]string{"WT_SESSION=1"}); got != want {
		t.Errorf("Detect() = %+v, want %+v", got, want)
	}
}
