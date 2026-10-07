//go:build !linux

package cpu

import (
	"testing"

	pscpu "github.com/shirou/gopsutil/v4/cpu"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestPortableFactsUseReportedProcessorCounts(t *testing.T) {
	t.Parallel()

	facts, err := portableFacts([]pscpu.InfoStat{{
		ModelName: "Example CPU",
		VendorID:  "Example",
		Mhz:       3200,
	}}, 12, 6)
	if err != nil {
		t.Fatal(err)
	}

	if got := facts[series.FactCPULogical]; got != "12" {
		t.Errorf("logical processors = %q, want 12", got)
	}
	if got := facts[series.FactCPUCores]; got != "6" {
		t.Errorf("cores = %q, want 6", got)
	}
	if got := facts[series.FactCPUSockets]; got != "1" {
		t.Errorf("sockets = %q, want 1", got)
	}
}

func TestPortableFactsRejectEmptyCPUInfo(t *testing.T) {
	t.Parallel()

	if _, err := portableFacts(nil, 0, 0); err == nil {
		t.Fatal("empty CPU information was accepted")
	}
}
