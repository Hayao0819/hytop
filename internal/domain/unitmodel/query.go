package unitmodel

import (
	"errors"
	"regexp"
	"strings"
)

// Query is a compiled case-insensitive search over unit fields.
type Query struct {
	source string
	terms  []string
	re     *regexp.Regexp
}

// Compile reads the search. Text wrapped in slashes is a regular expression;
// anything else is a list of words, all of which have to appear.
func Compile(text string) (Query, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Query{}, nil
	}

	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, "/") {
		pattern := trimmed[1 : len(trimmed)-1]
		if pattern == "" {
			return Query{}, errors.New("an empty regular expression matches everything; leave the search blank instead")
		}

		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return Query{}, err
		}

		return Query{source: text, re: re}, nil
	}

	return Query{source: text, terms: strings.Fields(strings.ToLower(trimmed))}, nil
}

// Source returns the original query text.
func (q Query) Source() string { return q.source }

func (q Query) Empty() bool { return q.re == nil && len(q.terms) == 0 }

// Matches searches a unit's name, description, and state fields.
func (q Query) Matches(unit Unit) bool {
	if q.Empty() {
		return true
	}

	haystack := unit.Name + " " + unit.Description + " " +
		unit.Load + " " + unit.Active + " " + unit.Sub

	if q.re != nil {
		return q.re.MatchString(haystack)
	}

	haystack = strings.ToLower(haystack)

	for _, term := range q.terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}

	return true
}
