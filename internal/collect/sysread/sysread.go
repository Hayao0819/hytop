// Package sysread reads the one-value text attributes exposed by sysfs.
package sysread

import (
	"os"
	"strconv"
	"strings"
)

// String returns a non-empty, whitespace-trimmed attribute.
func String(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	value := strings.TrimSpace(string(raw))

	return value, value != ""
}

// Text is String for optional attributes where absence is represented by an
// empty value.
func Text(path string) string {
	value, _ := String(path)

	return value
}

// Float parses a numeric attribute.
func Float(path string) (float64, bool) {
	value, ok := String(path)
	if !ok {
		return 0, false
	}

	n, err := strconv.ParseFloat(value, 64)

	return n, err == nil
}
