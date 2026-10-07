package collect_test

import (
	"errors"
	"testing"

	"github.com/Hayao0819/hytop/internal/collect"
)

func TestKernelAvailability(t *testing.T) {
	t.Parallel()

	if got := collect.KernelAvailability(nil, "repair"); got.State != collect.Ready {
		t.Fatalf("nil probe = %+v", got)
	}

	got := collect.KernelAvailability(errors.New("missing"), "repair")
	if got.State != collect.NoKernelSupport || got.Reason != "missing" || got.Remedy != "repair" {
		t.Fatalf("failed probe = %+v", got)
	}
}
