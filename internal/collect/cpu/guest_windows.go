package cpu

import (
	"context"

	"github.com/yusufpapurcu/wmi"

	"github.com/Hayao0819/hytop/internal/collect"
)

func platformGuestFacts(ctx context.Context) collect.Facts {
	if ctx.Err() != nil {
		return nil
	}
	var systems []struct{ Manufacturer, Model string }
	if err := wmi.Query("SELECT Manufacturer, Model FROM Win32_ComputerSystem", &systems); err != nil || len(systems) == 0 {
		return nil
	}
	return guestFacts(dmiHypervisor(systems[0].Manufacturer, systems[0].Model), "")
}
