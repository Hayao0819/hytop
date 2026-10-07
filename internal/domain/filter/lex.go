// Package filter is the process filter language.
//
// Evaluation returns PID sets, allowing structural operators such as
// descendants(...) and subtree(...).
package filter

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenOperator
	tokenLParen
	tokenRParen
	tokenVariable
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func (t token) String() string {
	if t.kind == tokenEOF {
		return "end of expression"
	}

	return fmt.Sprintf("%q", t.text)
}

// Longest first, so ">=" wins over ">".
var operators = []string{">=", "<=", "==", "!=", "^=", "~", ">", "<"}

type lexer struct {
	input string
	pos   int
}

func lex(input string) ([]token, error) {
	l := &lexer{input: input}

	var tokens []token

	for {
		t, err := l.next()
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, t)

		if t.kind == tokenEOF {
			return tokens, nil
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skipSpace()

	if l.pos >= len(l.input) {
		return token{kind: tokenEOF, pos: l.pos}, nil
	}

	start := l.pos
	c := l.input[l.pos]

	switch {
	case c == '(':
		l.pos++

		return token{kind: tokenLParen, text: "(", pos: start}, nil

	case c == ')':
		l.pos++

		return token{kind: tokenRParen, text: ")", pos: start}, nil

	case c == '"' || c == '\'':
		return l.lexString(c)

	case c == '$':
		l.pos++
		name := l.take(isIdentRune)

		if name == "" {
			return token{}, fmt.Errorf("filter: bare $ at %d", start)
		}

		return token{kind: tokenVariable, text: name, pos: start}, nil

	case isDigit(c):
		return l.lexNumber()
	}

	for _, op := range operators {
		if strings.HasPrefix(l.input[l.pos:], op) {
			l.pos += len(op)

			return token{kind: tokenOperator, text: op, pos: start}, nil
		}
	}

	if isIdentStart(rune(c)) {
		return token{kind: tokenIdent, text: l.take(isIdentRune), pos: start}, nil
	}

	return token{}, fmt.Errorf("filter: unexpected %q at %d", string(c), start)
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *lexer) take(keep func(rune) bool) string {
	start := l.pos

	for l.pos < len(l.input) && keep(rune(l.input[l.pos])) {
		l.pos++
	}

	return l.input[start:l.pos]
}

func (l *lexer) lexString(quote byte) (token, error) {
	start := l.pos
	l.pos++

	var value strings.Builder

	for l.pos < len(l.input) {
		c := l.input[l.pos]

		switch c {
		case '\\':
			if l.pos+1 >= len(l.input) {
				return token{}, fmt.Errorf("filter: trailing backslash at %d", l.pos)
			}

			l.pos++
			value.WriteByte(l.input[l.pos])
			l.pos++

		case quote:
			l.pos++

			return token{kind: tokenString, text: value.String(), pos: start}, nil

		default:
			value.WriteByte(c)
			l.pos++
		}
	}

	return token{}, fmt.Errorf("filter: unterminated string at %d", start)
}

// lexNumber keeps a size suffix in the token; the value is resolved at compare.
func (l *lexer) lexNumber() (token, error) {
	start := l.pos

	l.take(func(r rune) bool { return isDigit(byte(r)) || r == '.' })

	if l.pos < len(l.input) && isSizeSuffix(l.input[l.pos]) {
		l.pos++

		// "1Gi" and "1GiB" and "1GB" all mean the same thing here.
		for l.pos < len(l.input) && (l.input[l.pos] == 'i' || l.input[l.pos] == 'B') {
			l.pos++
		}
	}

	return token{kind: tokenNumber, text: l.input[start:l.pos], pos: start}, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isSizeSuffix(c byte) bool { return strings.IndexByte("kKmMgGtT", c) >= 0 }

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}
