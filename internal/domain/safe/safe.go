// Package safe sanitizes external text before hytop displays it.
package safe

import "github.com/Hayao0819/hytop/pkg/termui/text"

func Text(s string) string { return text.Line(s) }
