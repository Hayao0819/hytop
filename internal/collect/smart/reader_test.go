//go:build linux

package smart

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

func TestPeriodicUnprivilegedRefreshKeepsElevatedSMART(t *testing.T) {
	t.Parallel()

	reader := NewReader("/sys", "/dev")
	reader.ReplaceElevated([]diskmodel.Device{{
		Name: "sda", Model: "disk", Serial: "one", Size: 100,
		Health: diskmodel.Passed, Temperature: 42, Wear: 7,
	}})
	reader.publish([]diskmodel.Device{{
		Name: "sda", Model: "disk", Serial: "one", Size: 100, Wear: -1,
		Reason: "reading SMART needs privilege",
	}})

	got := reader.Devices()
	if len(got) != 1 || got[0].Health != diskmodel.Passed || got[0].Temperature != 42 || got[0].Wear != 7 {
		t.Fatalf("elevated SMART was overwritten: %+v", got)
	}
}

func TestElevatedSMARTIsNotReusedForAReplacementDrive(t *testing.T) {
	t.Parallel()

	reader := NewReader("/sys", "/dev")
	reader.ReplaceElevated([]diskmodel.Device{{
		Name: "sda", Model: "disk", Serial: "old", Size: 100, Health: diskmodel.Passed,
	}})
	reader.publish([]diskmodel.Device{{
		Name: "sda", Model: "disk", Serial: "new", Size: 100, Wear: -1,
		Reason: "reading SMART needs privilege",
	}})

	got := reader.Devices()
	if len(got) != 1 || got[0].Health != diskmodel.Unknown || got[0].Reason == "" {
		t.Fatalf("replacement drive inherited stale SMART: %+v", got)
	}
}
