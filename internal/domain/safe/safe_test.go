package safe_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/safe"
)

func TestOrdinaryTextIsLeftAlone(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"systemd", "/usr/bin/firefox --profile x", "日本語のファイル名", "emoji 🎉 ok", "",
	} {
		if got := safe.Text(text); got != text {
			t.Errorf("Text(%q) = %q, want it unchanged", text, got)
		}
	}
}

func TestEscapeSequencesCannotReachTheTerminal(t *testing.T) {
	t.Parallel()

	got := safe.Text("ok\x1b[2J\x1b[HWIPED")

	if strings.Contains(got, "\x1b") {
		t.Errorf("Text kept an escape: %q", got)
	}

	if !strings.Contains(got, "ok") || !strings.Contains(got, "WIPED") {
		t.Errorf("Text threw away the readable part: %q", got)
	}
}

func TestTheOneByteIntroducerIsCaughtToo(t *testing.T) {
	t.Parallel()

	if got := safe.Text("a\u009b2Jb"); strings.ContainsRune(got, 0x9b) {
		t.Errorf("Text kept a C1 introducer: %q", got)
	}
}

func TestLineBreaksBecomeSpaces(t *testing.T) {
	t.Parallel()

	got := safe.Text("first\rsecond\nthird\tfourth")

	if strings.ContainsAny(got, "\r\n\t") {
		t.Errorf("Text kept a line break: %q", got)
	}

	if got != "first second third fourth" {
		t.Errorf("Text(%q) = %q", "first\\rsecond\\nthird\\tfourth", got)
	}
}

func TestBytesThatAreNotUTF8AreReplaced(t *testing.T) {
	t.Parallel()

	if got := safe.Text("a\xffb"); got != "a?b" {
		t.Errorf("Text(\"a\\xffb\") = %q, want \"a?b\"", got)
	}
}

func TestReplacementRuneIsPreserved(t *testing.T) {
	t.Parallel()

	if got := safe.Text("a�b"); got != "a�b" {
		t.Errorf("Text changed a valid replacement rune to %q", got)
	}
}

func TestBidirectionalOverridesCannotReorderAField(t *testing.T) {
	t.Parallel()

	got := safe.Text("before\u202eexe.txt\u202cafter")
	if strings.ContainsAny(got, "\u202a\u202b\u202c\u202d\u202e") {
		t.Fatalf("Text kept a bidirectional override: %q", got)
	}
	if !strings.Contains(got, "exe.txt") {
		t.Fatalf("Text removed the readable part: %q", got)
	}
}
