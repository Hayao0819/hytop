package sysread_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect/sysread"
)

func TestStringAndFloat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attribute")
	if err := os.WriteFile(path, []byte(" 12.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, ok := sysread.String(path); !ok || got != "12.5" {
		t.Fatalf("String() = %q, %v", got, ok)
	}
	if got, ok := sysread.Float(path); !ok || got != 12.5 {
		t.Fatalf("Float() = %v, %v", got, ok)
	}
}
