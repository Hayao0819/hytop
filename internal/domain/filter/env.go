package filter

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// EnvVars are the runtime values the language defines: $USER, $PPID, $SELF.
func EnvVars() Vars {
	vars := Vars{
		"SELF": strconv.Itoa(os.Getpid()),
		"PPID": strconv.Itoa(os.Getppid()),
	}

	if name := os.Getenv("USER"); name != "" {
		vars["USER"] = name
	}

	return vars
}

// Compile is Parse against EnvVars. An empty expression selects everything, so
// clearing the filter bar clears the filter.
func Compile(src string) (Expr, error) {
	if src == "" {
		return nil, nil
	}

	legacy, legacyErr := Parse(src, EnvVars())
	if legacyErr == nil {
		return legacy, nil
	}

	program, programErr := compileProgram(src)
	if programErr == nil {
		return program, nil
	}

	if strings.HasPrefix(strings.TrimSpace(src), "expr(") {
		return nil, legacyErr
	}

	return nil, errors.Join(legacyErr, programErr)
}
