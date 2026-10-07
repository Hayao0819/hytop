package unitmodel_test

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
)

func unit() unitmodel.Unit {
	return unitmodel.Unit{
		Name:        "sshd.service",
		Description: "OpenBSD Secure Shell server",
		Load:        "loaded",
		Active:      "failed",
		Sub:         "failed",
	}
}

func TestAnEmptySearchKeepsEverything(t *testing.T) {
	t.Parallel()

	query, err := unitmodel.Compile("   ")
	if err != nil {
		t.Fatal(err)
	}

	if !query.Empty() || !query.Matches(unitmodel.Unit{}) {
		t.Error("an empty search should keep everything")
	}
}

func TestEveryWordHasToAppear(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"ssh":          true,
		"SSH":          true,
		"shell server": true,
		"failed ssh":   true,
		"ssh nginx":    false,
		"nginx":        false,
	}

	for text, want := range cases {
		query, err := unitmodel.Compile(text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}

		if got := query.Matches(unit()); got != want {
			t.Errorf("%q matched %v, want %v", text, got, want)
		}
	}
}

func TestSlashesMakeItARegularExpression(t *testing.T) {
	t.Parallel()

	query, err := unitmodel.Compile(`/^sshd\.(service|socket)$/`)
	if err != nil {
		t.Fatal(err)
	}

	if query.Matches(unit()) {
		t.Error("the expression is anchored, and the haystack has more in it than the name")
	}

	query, err = unitmodel.Compile(`/sshd\.service/`)
	if err != nil {
		t.Fatal(err)
	}

	if !query.Matches(unit()) {
		t.Error("the expression should have matched")
	}

	if _, err := unitmodel.Compile("/(unclosed/"); err == nil {
		t.Error("a broken expression should be refused")
	}

	if _, err := unitmodel.Compile("//"); err == nil {
		t.Error("an empty expression should be refused")
	}
}

func TestPlainSlashesAreNotAnExpression(t *testing.T) {
	t.Parallel()

	query, err := unitmodel.Compile("/etc")
	if err != nil {
		t.Fatal(err)
	}

	if query.Matches(unit()) {
		t.Error("/etc should be looked for as text")
	}
}
