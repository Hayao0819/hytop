package series

import (
	"fmt"
	"slices"
	"strings"
)

// Def describes one series or a family of them. A "{name}" segment stands for
// one segment and captures it.
type Def struct {
	Template string
	Unit     Unit
	Help     string
}

// Registry holds explicitly registered definitions. Its zero value is ready to use.
type Registry struct {
	defs []Def
}

// Register rejects a duplicate template: two collectors claiming one key would
// make its unit ambiguous.
func (r *Registry) Register(defs ...Def) error {
	for _, def := range defs {
		if err := validTemplate(def.Template); err != nil {
			return err
		}

		for _, existing := range r.defs {
			if existing.Template == def.Template {
				return fmt.Errorf("series: %q registered twice", def.Template)
			}
		}

		r.defs = append(r.defs, def)
	}

	return nil
}

func (r *Registry) MustRegister(defs ...Def) {
	if err := r.Register(defs...); err != nil {
		panic(err)
	}
}

func (r *Registry) Defs() []Def { return slices.Clone(r.defs) }

func (r *Registry) Lookup(k Key) (Def, map[string]string, bool) {
	for _, def := range r.defs {
		if captured, ok := capture(def.Template, k); ok {
			return def, captured, true
		}
	}

	return Def{}, nil, false
}

// Unit returns k's unit, or None when k is unregistered.
func (r *Registry) Unit(k Key) Unit {
	def, _, ok := r.Lookup(k)
	if !ok {
		return None
	}

	return def.Unit
}

// PatternUnit returns the common unit of every definition a selector can
// reach. False means the selector is unknown or spans incompatible units.
func (r *Registry) PatternUnit(pattern Pattern) (Unit, bool) {
	if _, err := ParsePattern(string(pattern)); err != nil {
		return None, false
	}

	var (
		unit  Unit
		found bool
	)

	for _, def := range r.defs {
		if !patternsIntersect(pattern, def.Pattern()) {
			continue
		}
		if found && def.Unit != unit {
			return None, false
		}
		unit, found = def.Unit, true
	}

	return unit, found
}

func patternsIntersect(a, b Pattern) bool {
	left, right := strings.Split(string(a), sep), strings.Split(string(b), sep)
	if len(left) > 0 && left[len(left)-1] == rest {
		return prefixCompatible(left[:len(left)-1], right) && len(right) >= len(left)
	}
	if len(right) > 0 && right[len(right)-1] == rest {
		return prefixCompatible(right[:len(right)-1], left) && len(left) >= len(right)
	}
	if len(left) != len(right) {
		return false
	}

	return prefixCompatible(left, right)
}

func prefixCompatible(prefix, value []string) bool {
	if len(prefix) > len(value) {
		return false
	}

	for i, segment := range prefix {
		if segment != one && value[i] != one && segment != value[i] {
			return false
		}
	}

	return true
}

func (d Def) Pattern() Pattern {
	segments := strings.Split(d.Template, sep)

	for i, segment := range segments {
		if placeholder(segment) != "" {
			segments[i] = one
		}
	}

	return Pattern(strings.Join(segments, sep))
}

func validTemplate(t string) error {
	if t == "" {
		return fmt.Errorf("series: empty template")
	}

	for _, segment := range strings.Split(t, sep) {
		switch {
		case segment == "":
			return fmt.Errorf("series: empty segment in %q", t)
		case strings.Contains(segment, one):
			return fmt.Errorf("series: wildcard in template %q; use {name}", t)
		case strings.ContainsAny(segment, "{}") && placeholder(segment) == "":
			return fmt.Errorf("series: malformed placeholder in %q", t)
		}
	}

	return nil
}

func placeholder(segment string) string {
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return ""
	}

	name := segment[1 : len(segment)-1]
	if strings.ContainsAny(name, "{}") {
		return ""
	}

	return name
}

func capture(t string, k Key) (map[string]string, bool) {
	var (
		want = strings.Split(t, sep)
		got  = k.Segments()
	)

	if len(want) != len(got) {
		return nil, false
	}

	var captured map[string]string

	for i, segment := range want {
		name := placeholder(segment)
		if name == "" {
			if segment != got[i] {
				return nil, false
			}

			continue
		}

		if captured == nil {
			captured = make(map[string]string, len(want))
		}

		captured[name] = got[i]
	}

	return captured, true
}
