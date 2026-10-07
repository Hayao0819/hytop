// Package errors exposes the project's stack-aware error helpers.
package errors

import (
	stderrors "errors"
	"fmt"

	stackerrors "github.com/go-errors/errors"
)

func New(msg string) error { return stackerrors.New(msg) }

func Newf(format string, args ...any) error { return stackerrors.Errorf(format, args...) }

// Wrap captures a stack only when err carries none: a second capture on the way
// up prints the same frames twice and buries the origin.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}

	return stackerrors.WrapPrefix(err, msg, 1)
}

func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	return stackerrors.WrapPrefix(err, fmt.Sprintf(format, args...), 1)
}

func Is(err, target error) bool { return stderrors.Is(err, target) }

func As(err error, target any) bool { return stderrors.As(err, target) }

// Details renders err with its stack, for the top level of a command.
func Details(err error) string {
	if err == nil {
		return ""
	}

	var stacked *stackerrors.Error
	if stderrors.As(err, &stacked) {
		return stacked.ErrorStack()
	}

	return fmt.Sprintf("%+v", err)
}
