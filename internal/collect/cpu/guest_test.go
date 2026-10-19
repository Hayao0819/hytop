package cpu

import (
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestDMIHypervisorDistinguishesMicrosoftGuestsFromPhysicalMachines(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ vendor, product, want string }{
		{"Microsoft Corporation", "Virtual Machine", "Hyper-V"},
		{"Microsoft Corporation", "Surface Pro", ""},
		{"Dell Inc.", "Precision", ""},
		{"QEMU", "Standard PC", "QEMU"},
		{"QEMU", "KVM", "KVM"},
		{"VMware, Inc.", "VMware Virtual Platform", "VMware"},
		{"innotek GmbH", "VirtualBox", "VirtualBox"},
		{"Xen", "HVM domU", "Xen"},
	} {
		if got := dmiHypervisor(test.vendor, test.product); got != test.want {
			t.Errorf("%s/%s = %q, want %q", test.vendor, test.product, got, test.want)
		}
	}
}

func TestConfidentialGuestDoesNotInventAHypervisor(t *testing.T) {
	t.Parallel()

	facts := guestFacts("", "Intel TDX")
	if facts[series.FactCPUEnvironment] != "VM guest" || facts[series.FactCPUHypervisor] != "Unknown" ||
		facts[series.FactCPUConfidential] != "Intel TDX" {
		t.Fatalf("confidential guest facts = %v", facts)
	}
	if facts := guestFacts("", ""); len(facts) != 0 {
		t.Fatalf("absent VM evidence produced facts: %v", facts)
	}
}
