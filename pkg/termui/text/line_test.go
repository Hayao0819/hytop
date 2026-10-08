package text_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Hayao0819/hytop/pkg/termui/text"
)

func TestOrdinaryTextIsLeftAlone(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"systemd", "/usr/bin/firefox --profile x", "日本語のファイル名", "emoji 🎉 ok", "",
	} {
		if got := text.Line(input); got != input {
			t.Errorf("Line(%q) = %q, want it unchanged", input, got)
		}
	}
}

func TestEscapeSequencesCannotReachTheTerminal(t *testing.T) {
	t.Parallel()

	got := text.Line("ok\x1b[2J\x1b[HWIPED")

	if strings.Contains(got, "\x1b") {
		t.Errorf("Line kept an escape: %q", got)
	}

	if !strings.Contains(got, "ok") || !strings.Contains(got, "WIPED") {
		t.Errorf("Line threw away the readable part: %q", got)
	}
}

func TestTheOneByteIntroducerIsCaughtToo(t *testing.T) {
	t.Parallel()

	if got := text.Line("a\u009b2Jb"); strings.ContainsRune(got, 0x9b) {
		t.Errorf("Line kept a C1 introducer: %q", got)
	}
}

func TestLineBreaksBecomeSpaces(t *testing.T) {
	t.Parallel()

	got := text.Line("first\rsecond\nthird\tfourth")

	if strings.ContainsAny(got, "\r\n\t") {
		t.Errorf("Line kept a line break: %q", got)
	}

	if got != "first second third fourth" {
		t.Errorf("Line(%q) = %q", "first\\rsecond\\nthird\\tfourth", got)
	}
}

func TestBytesThatAreNotUTF8AreReplaced(t *testing.T) {
	t.Parallel()

	if got := text.Line("a\xffb"); got != "a?b" {
		t.Errorf("Line(\"a\\xffb\") = %q, want \"a?b\"", got)
	}
}

func TestReplacementRuneIsPreserved(t *testing.T) {
	t.Parallel()

	if got := text.Line("a�b"); got != "a�b" {
		t.Errorf("Line changed a valid replacement rune to %q", got)
	}
}

func TestBidirectionalOverridesCannotReorderAField(t *testing.T) {
	t.Parallel()

	got := text.Line("before\u202eexe.txt\u202cafter")
	if strings.ContainsAny(got, "\u202a\u202b\u202c\u202d\u202e") {
		t.Fatalf("Line kept a bidirectional override: %q", got)
	}
	if !strings.Contains(got, "exe.txt") {
		t.Fatalf("Line removed the readable part: %q", got)
	}
}

func FuzzLine(f *testing.F) {
	for _, input := range []string{"hello 日本語", "\x1b[31mred\x1b[0m", "a\xffb", "\r\n\t", "\u202ebidi"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := text.Line(input)
		if !utf8.ValidString(got) || text.Line(got) != got {
			t.Fatalf("Line(%q) = %q: invalid UTF-8 or not idempotent", input, got)
		}
		for _, r := range got {
			if r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == 0x061c ||
				r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) ||
				(r >= 0x2066 && r <= 0x206f) {
				t.Fatalf("Line kept control U+%04X in %q", r, got)
			}
		}
	})
}

func ExampleLine() {
	fmt.Println(text.Line("job\n\x1b[2Jdone"))
	// Output: job ?[2Jdone
}
