package errors_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/errors"
)

func TestDetailsCarriesTheStackFromWhereItStarted(t *testing.T) {
	t.Parallel()

	err := errors.Wrap(leaf(), "reading /proc")

	details := errors.Details(err)

	if !strings.Contains(details, "leaf") {
		t.Errorf("the frame that failed is missing:\n%s", details)
	}

	if !strings.Contains(details, "reading /proc") {
		t.Errorf("the added context is missing:\n%s", details)
	}
}

func leaf() error { return errors.New("no such thing") }

func TestWrappingTwiceCapturesOneStack(t *testing.T) {
	t.Parallel()

	once := errors.Details(errors.Wrap(leaf(), "one"))
	twice := errors.Details(errors.Wrap(errors.Wrap(leaf(), "one"), "two"))

	if strings.Count(twice, "leaf") > strings.Count(once, "leaf") {
		t.Errorf("the leaf frame appears more than once:\n%s", twice)
	}
}

func TestWrappingAForeignErrorAddsAStack(t *testing.T) {
	t.Parallel()

	_, err := os.Open("/nonexistent/hytop")

	wrapped := errors.Wrap(err, "opening the thing")

	if !errors.Is(wrapped, fs.ErrNotExist) {
		t.Error("Is lost the cause")
	}

	if !strings.Contains(errors.Details(wrapped), "TestWrappingAForeignErrorAddsAStack") {
		t.Errorf("no stack was added:\n%s", errors.Details(wrapped))
	}
}

func TestWrappingNilIsNil(t *testing.T) {
	t.Parallel()

	if errors.Wrap(nil, "x") != nil || errors.Wrapf(nil, "%s", "x") != nil {
		t.Error("wrapping nil produced an error")
	}

	if got := errors.Details(nil); got != "" {
		t.Errorf("Details(nil) = %q", got)
	}
}
