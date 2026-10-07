package filter

import (
	"strings"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type field string

type valueKind = procmodel.FieldKind

const (
	kindNumber = procmodel.NumberField
	kindString = procmodel.StringField
	kindBool   = procmodel.BoolField
)

func Fields() []string { return procmodel.Fields() }

func (f field) model() procmodel.Field {
	model, _ := procmodel.LookupField(string(f))

	return model
}

func (f field) kind() valueKind { return f.model().Kind }

func (f field) truth(p *procmodel.Process) bool { return f.model().Bool(p) }

func (e *compareExpr) matches(p *procmodel.Process) bool {
	field := e.field.model()
	switch field.Kind {
	case procmodel.BoolField:
		return e.matchBool(field.Bool(p))
	case procmodel.StringField:
		return e.matchString(field.Text(p))
	default:
		return e.matchNumber(field.Numbers(p))
	}
}

func (e *compareExpr) matchBool(got bool) bool {
	want := e.text == "true" || e.text == "1" || e.text == "yes"
	if e.operator == "!=" {
		return got != want
	}

	return got == want
}

func (e *compareExpr) matchString(got string) bool {
	switch e.operator {
	case "==":
		return got == e.text
	case "!=":
		return got != e.text
	case "~":
		return e.pattern.MatchString(got)
	case "^=":
		return strings.HasPrefix(got, e.text)
	default:
		return false
	}
}

func (e *compareExpr) matchNumber(got []float64) bool {
	if len(got) == 0 {
		return e.operator == "!="
	}

	for _, value := range got {
		if compare(value, e.operator, e.number) {
			return true
		}
	}

	return false
}

func compare(got float64, operator string, want float64) bool {
	switch operator {
	case "==":
		return got == want
	case "!=":
		return got != want
	case ">":
		return got > want
	case ">=":
		return got >= want
	case "<":
		return got < want
	case "<=":
		return got <= want
	default:
		return false
	}
}
