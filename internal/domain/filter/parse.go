package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type Expr interface {
	fmt.Stringer

	eval(*scope) set
}

// Each relation walks the tree from the set its argument selected.
type relation int

const (
	relChildren relation = iota
	relDescendants
	relSubtree
	relAncestors
	relSiblings
)

var relationNames = map[string]relation{
	"children":    relChildren,
	"descendants": relDescendants,
	"subtree":     relSubtree,
	"ancestors":   relAncestors,
	"siblings":    relSiblings,
}

func (r relation) String() string {
	for name, kind := range relationNames {
		if kind == r {
			return name
		}
	}

	return "?"
}

type (
	andExpr struct{ left, right Expr }
	orExpr  struct{ left, right Expr }
	notExpr struct{ inner Expr }

	relExpr struct {
		kind  relation
		inner Expr
	}

	compareExpr struct {
		field    field
		operator string
		text     string
		number   float64
		pattern  *regexp.Regexp
	}

	truthExpr struct{ field field }
)

func (e *andExpr) String() string { return "(" + e.left.String() + " and " + e.right.String() + ")" }

func (e *orExpr) String() string    { return "(" + e.left.String() + " or " + e.right.String() + ")" }
func (e *notExpr) String() string   { return "not " + e.inner.String() }
func (e *relExpr) String() string   { return e.kind.String() + "(" + e.inner.String() + ")" }
func (e *truthExpr) String() string { return string(e.field) }

func (e *compareExpr) String() string {
	value := e.text
	if e.field.kind() != kindNumber {
		value = strconv.Quote(value)
	}

	return string(e.field) + " " + e.operator + " " + value
}

type Vars map[string]string

// Parse resolves variables now, so naming an unset one fails here rather than
// silently matching nothing.
func Parse(src string, vars Vars) (Expr, error) {
	tokens, err := lex(src)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens, vars: vars}

	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if got := p.peek(); got.kind != tokenEOF {
		return nil, fmt.Errorf("filter: unexpected %s at %d", got, got.pos)
	}

	return expr, nil
}

type parser struct {
	tokens []token
	pos    int
	vars   Vars
}

func (p *parser) peek() token { return p.tokens[p.pos] }

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	if t.kind != tokenEOF {
		p.pos++
	}

	return t
}

func (p *parser) acceptKeyword(word string) bool {
	if t := p.peek(); t.kind == tokenIdent && t.text == word {
		p.pos++

		return true
	}

	return false
}

func (p *parser) parseOr() (Expr, error) {
	return p.parseBinary(p.parseAnd, "or", func(left, right Expr) Expr {
		return &orExpr{left: left, right: right}
	})
}

func (p *parser) parseAnd() (Expr, error) {
	return p.parseBinary(p.parseNot, "and", func(left, right Expr) Expr {
		return &andExpr{left: left, right: right}
	})
}

// parseBinary implements one left-associative precedence level. The caller's
// next function is the tighter level in the grammar.
func (p *parser) parseBinary(next func() (Expr, error), keyword string, join func(Expr, Expr) Expr) (Expr, error) {
	left, err := next()
	if err != nil {
		return nil, err
	}

	for p.acceptKeyword(keyword) {
		right, err := next()
		if err != nil {
			return nil, err
		}

		left = join(left, right)
	}

	return left, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.acceptKeyword("not") {
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}

		return &notExpr{inner: inner}, nil
	}

	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()

	if t.kind == tokenLParen {
		p.advance()

		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}

		if got := p.advance(); got.kind != tokenRParen {
			return nil, fmt.Errorf("filter: want ) at %d, got %s", got.pos, got)
		}

		return inner, nil
	}

	if t.kind != tokenIdent {
		return nil, fmt.Errorf("filter: want a field or a function at %d, got %s", t.pos, t)
	}

	if kind, ok := relationNames[t.text]; ok && p.tokens[p.pos+1].kind == tokenLParen {
		return p.parseRelation(kind)
	}
	if t.text == "expr" && p.tokens[p.pos+1].kind == tokenLParen {
		return p.parseProgram()
	}

	return p.parseComparison()
}

func (p *parser) parseProgram() (Expr, error) {
	p.advance() // expr
	p.advance() // (

	source := p.advance()
	if source.kind != tokenString {
		return nil, fmt.Errorf("filter: expr needs a quoted Expr expression at %d", source.pos)
	}
	if got := p.advance(); got.kind != tokenRParen {
		return nil, fmt.Errorf("filter: want ) at %d, got %s", got.pos, got)
	}

	return compileProgram(source.text)
}

func (p *parser) parseRelation(kind relation) (Expr, error) {
	p.advance() // name
	p.advance() // (

	inner, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if got := p.advance(); got.kind != tokenRParen {
		return nil, fmt.Errorf("filter: want ) at %d, got %s", got.pos, got)
	}

	return &relExpr{kind: kind, inner: inner}, nil
}

func (p *parser) parseComparison() (Expr, error) {
	name := p.advance()

	_, ok := procmodel.LookupField(name.text)
	if !ok {
		return nil, fmt.Errorf("filter: unknown field %q at %d", name.text, name.pos)
	}
	f := field(name.text)

	if p.peek().kind != tokenOperator {
		if f.kind() != kindBool {
			return nil, fmt.Errorf("filter: %q needs a comparison at %d", name.text, name.pos)
		}

		return &truthExpr{field: f}, nil
	}

	operator := p.advance()

	value := p.advance()

	text, err := p.literal(value)
	if err != nil {
		return nil, err
	}

	return newCompare(f, operator.text, text)
}

func (p *parser) literal(t token) (string, error) {
	switch t.kind {
	case tokenString, tokenNumber:
		return t.text, nil

	case tokenIdent:
		return t.text, nil

	case tokenVariable:
		value, ok := p.vars[t.text]
		if !ok {
			return "", fmt.Errorf("filter: $%s is not set", t.text)
		}

		return value, nil

	default:
		return "", fmt.Errorf("filter: want a value at %d, got %s", t.pos, t)
	}
}

func newCompare(f field, operator, text string) (Expr, error) {
	e := &compareExpr{field: f, operator: operator, text: text}

	switch operator {
	case "~":
		if f.kind() != kindString {
			return nil, fmt.Errorf("filter: ~ needs a text field, not %q", f)
		}

		pattern, err := regexp.Compile(text)
		if err != nil {
			return nil, fmt.Errorf("filter: bad regexp %q: %w", text, err)
		}

		e.pattern = pattern

	case "^=":
		if f.kind() != kindString {
			return nil, fmt.Errorf("filter: ^= needs a text field, not %q", f)
		}

	case "<", "<=", ">", ">=":
		if f.kind() != kindNumber {
			return nil, fmt.Errorf("filter: %s needs a number, not %q", operator, f)
		}
	}

	if f.kind() == kindNumber && e.pattern == nil {
		n, err := parseNumber(text)
		if err != nil {
			return nil, err
		}

		e.number = n
	}

	return e, nil
}

var sizeUnits = map[byte]float64{
	'k': 1 << 10, 'K': 1 << 10,
	'm': 1 << 20, 'M': 1 << 20,
	'g': 1 << 30, 'G': 1 << 30,
	't': 1 << 40, 'T': 1 << 40,
}

func parseNumber(text string) (float64, error) {
	digits := strings.TrimRight(text, "iB")

	multiplier := 1.0

	if len(digits) > 0 {
		if unit, ok := sizeUnits[digits[len(digits)-1]]; ok {
			multiplier = unit
			digits = digits[:len(digits)-1]
		}
	}

	n, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, fmt.Errorf("filter: %q is not a number", text)
	}

	return n * multiplier, nil
}
