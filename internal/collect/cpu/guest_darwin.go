package cpu

import (
	"context"

	"golang.org/x/sys/unix"

	"github.com/Hayao0819/hytop/internal/collect"
)

func platformGuestFacts(context.Context) collect.Facts {
	if present, err := unix.SysctlUint32("kern.hv_vmm_present"); err == nil && present != 0 {
		return guestFacts("Unknown", "")
	}
	return nil
}
