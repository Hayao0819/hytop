package journal

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConsumeReportsAnOversizedJournalEntry(t *testing.T) {
	t.Parallel()

	reader := NewReader(10)
	stopped := false
	reader.consume(context.Background(), strings.NewReader(strings.Repeat("x", (1<<20)+1)), func() {
		stopped = true
	}, func() error {
		return nil
	})

	if err := reader.Err(); err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("scanner error = %v", err)
	}
	if !stopped {
		t.Error("journalctl was left running after its output became unreadable")
	}
}

func TestConsumeReportsJournalctlFailure(t *testing.T) {
	t.Parallel()

	reader := NewReader(10)
	reader.consume(context.Background(), strings.NewReader(""), func() {}, func() error {
		return errors.New("access denied")
	})

	if err := reader.Err(); err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("journalctl error = %v", err)
	}
}

func TestConsumeIgnoresExitErrorAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := NewReader(10)
	reader.consume(ctx, strings.NewReader(""), func() {}, func() error { return errors.New("signal: killed") })

	if err := reader.Err(); err != nil {
		t.Fatalf("canceled journal reported %v", err)
	}
}
