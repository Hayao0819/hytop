package cpu

import (
	"strings"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func guestFacts(hypervisor, confidential string) collect.Facts {
	if hypervisor == "" && confidential == "" {
		return nil
	}
	if hypervisor == "" {
		hypervisor = "Unknown"
	}
	facts := collect.Facts{
		series.FactCPUEnvironment: "VM guest",
		series.FactCPUHypervisor:  hypervisor,
	}
	if confidential != "" {
		facts[series.FactCPUConfidential] = confidential
	}
	return facts
}

func dmiHypervisor(vendor, product string) string {
	if strings.EqualFold(strings.TrimSpace(vendor), "Microsoft Corporation") &&
		strings.EqualFold(strings.TrimSpace(product), "Virtual Machine") {
		return "Hyper-V"
	}
	for _, value := range []string{product, vendor} {
		value = strings.ToLower(strings.TrimSpace(value))
		for _, known := range []struct{ prefix, name string }{
			{"kvm", "KVM"},
			{"qemu", "QEMU"},
			{"vmware", "VMware"},
			{"virtualbox", "VirtualBox"},
			{"innotek", "VirtualBox"},
			{"xen", "Xen"},
			{"parallels", "Parallels"},
			{"bhyve", "bhyve"},
			{"hyper-v", "Hyper-V"},
			{"openstack", "KVM"},
		} {
			if strings.HasPrefix(value, known.prefix) {
				return known.name
			}
		}
	}
	return ""
}
