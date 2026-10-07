package app_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/app"
)

func TestEveryDRMCardGetsANamedGPUPage(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixtureWith(t, func(env *app.Env) {
		env.GPUs = func() []int { return []int{0, 1} }
	})
	memory.WriteFacts(map[string]string{
		"gpu.name.0": "Radeon RX Vega 64",
		"gpu.name.1": "GeForce RTX 3060",
	})
	tick(program)

	got := plain(program)
	for _, want := range []string{"Radeon RX", "GeForce RTX"} {
		if !strings.Contains(got, want) {
			t.Fatalf("GPU rail is missing %q:\n%s", want, got)
		}
	}

	for range 10 {
		if program.Route() == "/graphs/gpu/1" {
			break
		}
		press(t, program, "j")
	}
	if program.Route() != "/graphs/gpu/1" {
		t.Fatalf("could not reach the second GPU, stopped at %s", program.Route())
	}
	got = plain(program)
	if !strings.Contains(got, "GeForce RTX 3060") || !strings.Contains(got, "Card") {
		t.Fatalf("second GPU page is not usable:\n%s", got)
	}
}
