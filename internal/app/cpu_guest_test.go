package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestCPUGuestFactsAppearAfterCollectionAndSurviveCoreViewAndResizing(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ hypervisor, confidential string }{
		{"KVM", "Intel TDX"}, {"Hyper-V", "AMD SEV-SNP"},
	} {
		program, memory, _ := fixture(t)
		memory.WriteFacts(map[string]string{
			series.FactCPUEnvironment: "VM guest", series.FactCPUHypervisor: test.hypervisor,
			series.FactCPUConfidential: test.confidential,
		})
		tick(program)
		for _, size := range [][2]int{{100, 28}, {140, 38}, {80, 32}} {
			program.Send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for range 2 {
				got := plain(program)
				for _, want := range []string{"Environment", "VM guest", "Hypervisor", test.hypervisor, "Confidential VM", test.confidential, "AMD-V"} {
					if !strings.Contains(got, want) {
						t.Fatalf("%dx%d: CPU guest page is missing %q:\n%s", size[0], size[1], want, got)
					}
				}
				press(t, program, "c")
			}
		}
	}
}
