// Package series defines measurement keys, units, and registry metadata.
package series

import (
	"fmt"
	"strings"
)

// Key names one measurement, as dotted segments: "cpu.core.3.usage".
type Key string

// Pattern matches keys. A "*" segment stands for exactly one segment, so
// "cpu.core.*.usage" matches "cpu.core.3.usage" but not "cpu.core.3.a.usage".
// A trailing "**" stands for one or more segments.
type Pattern string

const (
	sep  = "."
	one  = "*"
	rest = "**"
)

func ParseKey(s string) (Key, error) {
	if s == "" {
		return "", fmt.Errorf("series: empty key")
	}

	for _, segment := range strings.Split(s, sep) {
		switch {
		case segment == "":
			return "", fmt.Errorf("series: empty segment in %q", s)
		case strings.Contains(segment, one):
			return "", fmt.Errorf("series: wildcard in key %q", s)
		case strings.ContainsAny(segment, "{}"):
			return "", fmt.Errorf("series: placeholder in key %q", s)
		}
	}

	return Key(s), nil
}

// ParsePattern validates a configured series selector. Wildcards occupy whole
// segments, and ** is meaningful only at the end.
func ParsePattern(s string) (Pattern, error) {
	if s == "" {
		return "", fmt.Errorf("series: empty pattern")
	}

	segments := strings.Split(s, sep)
	for i, segment := range segments {
		switch {
		case segment == "":
			return "", fmt.Errorf("series: empty segment in pattern %q", s)
		case segment == rest && i != len(segments)-1:
			return "", fmt.Errorf("series: ** must be the last segment in pattern %q", s)
		case strings.Contains(segment, one) && segment != one && segment != rest:
			return "", fmt.Errorf("series: wildcard must fill a segment in pattern %q", s)
		case strings.ContainsAny(segment, "{}"):
			return "", fmt.Errorf("series: placeholder in pattern %q", s)
		}
	}

	return Pattern(s), nil
}

func (k Key) String() string { return string(k) }

func (k Key) Segments() []string { return strings.Split(string(k), sep) }

// NormalizeSegment percent-encodes punctuation and UTF-8 bytes so an external
// name occupies exactly one segment without colliding with another name.
func NormalizeSegment(name string) string {
	const hex = "0123456789ABCDEF"

	var out strings.Builder
	for i := range len(name) {
		c := name[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hex[c>>4])
		out.WriteByte(hex[c&0x0f])
	}

	return out.String()
}

func (p Pattern) Match(k Key) bool {
	return matchSegments(strings.Split(string(p), sep), k.Segments())
}

func (p Pattern) Expand(live []Key) []Key {
	var matched []Key

	for _, key := range live {
		if p.Match(key) {
			matched = append(matched, key)
		}
	}

	return matched
}

func matchSegments(pattern, key []string) bool {
	for i, want := range pattern {
		if want == rest {
			return i == len(pattern)-1 && i < len(key)
		}

		if i >= len(key) {
			return false
		}

		if want != one && want != key[i] {
			return false
		}
	}

	return len(pattern) == len(key)
}
