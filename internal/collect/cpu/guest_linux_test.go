package cpu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestLinuxGuestFactsUseActiveGuestEvidence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, flags, hypervisor, confidential string
		files                                 map[string]string
		dirs                                  []string
	}{
		{name: "bare metal", flags: "vmx"},
		{name: "SEV capable host", flags: "svm sev sev_es sev_snp"},
		{name: "KVM host modules", flags: "svm", files: map[string]string{"module/kvm/parameters/enable_virt_at_load": "Y"}},
		{name: "KVM guest", hypervisor: "KVM", files: map[string]string{"class/dmi/id/product_name": "KVM"}},
		{name: "QEMU guest", hypervisor: "QEMU", files: map[string]string{"class/dmi/id/sys_vendor": "QEMU", "class/dmi/id/product_name": "Standard PC"}},
		{name: "Hyper-V guest", hypervisor: "Hyper-V", files: map[string]string{"class/dmi/id/sys_vendor": "Microsoft Corporation", "class/dmi/id/product_name": "Virtual Machine"}},
		{name: "Microsoft physical machine", files: map[string]string{"class/dmi/id/sys_vendor": "Microsoft Corporation", "class/dmi/id/product_name": "Surface"}},
		{name: "VMware guest", hypervisor: "VMware", files: map[string]string{"class/dmi/id/product_name": "VMware Virtual Platform"}},
		{name: "Xen guest", hypervisor: "Xen", files: map[string]string{"hypervisor/type": "xen"}},
		{name: "unknown hypervisor", flags: "hypervisor", hypervisor: "Unknown"},
		{name: "TDX flag", flags: "tdx_guest", hypervisor: "Unknown", confidential: "Intel TDX"},
		{name: "TDX guest device", dirs: []string{"class/misc/tdx_guest"}, hypervisor: "Unknown", confidential: "Intel TDX"},
		{name: "SNP guest device", dirs: []string{"devices/platform/sev-guest"}, hypervisor: "Unknown", confidential: "AMD SEV-SNP"},
		{name: "SNP VMPL", files: map[string]string{"devices/system/cpu/sev/vmpl": "0"}, hypervisor: "Unknown", confidential: "AMD SEV-SNP"},
	} {
		t.Run(test.name, func(t *testing.T) {
			procRoot, sysRoot := t.TempDir(), t.TempDir()
			write(t, filepath.Join(procRoot, "cpuinfo"), "processor: 0\nmodel name: Fixture\nflags: "+test.flags+"\n")
			for path, value := range test.files {
				write(t, filepath.Join(sysRoot, path), value)
			}
			for _, dir := range test.dirs {
				if err := os.MkdirAll(filepath.Join(sysRoot, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			collector, err := New(procRoot, sysRoot)
			if err != nil {
				t.Fatal(err)
			}
			if collector.detectVirt != nil {
				t.Fatal("alternate roots must not probe the running host")
			}
			facts, err := collector.Facts(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if facts[series.FactCPUHypervisor] != test.hypervisor || facts[series.FactCPUConfidential] != test.confidential {
				t.Fatalf("guest facts = %v, want hypervisor=%q confidential=%q", facts, test.hypervisor, test.confidential)
			}
			if (facts[series.FactCPUEnvironment] == "VM guest") != (test.hypervisor != "") {
				t.Fatalf("guest state = %q", facts[series.FactCPUEnvironment])
			}
		})
	}
}

func TestSystemdGuestFactsSeparateVMsAndConfidentialTechnology(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ vm, cvm, hypervisor, confidential string }{
		{"kvm", "tdx", "KVM", "Intel TDX"},
		{"microsoft", "sev-snp", "Hyper-V", "AMD SEV-SNP"},
		{"kvm", "sev-es", "KVM", "AMD SEV-ES"},
		{"kvm", "sev", "KVM", "AMD SEV"},
		{"vmware", "none", "VMware", ""},
		{"none", "none", "", ""},
	} {
		collector := &Collector{procRoot: t.TempDir(), sysRoot: t.TempDir()}
		collector.detectVirt = func(_ context.Context, flag string) (string, error) {
			if flag == "--vm" {
				return test.vm, nil
			}
			if flag != "--cvm" {
				t.Fatalf("unexpected detector argument %q", flag)
			}
			return test.cvm, nil
		}
		facts := collector.guestFacts(t.Context(), []string{"hypervisor"})
		if facts[series.FactCPUHypervisor] != test.hypervisor || facts[series.FactCPUConfidential] != test.confidential {
			t.Fatalf("%s/%s: facts = %v", test.vm, test.cvm, facts)
		}
	}
}

func TestUnavailableDetectorPreservesSysfsFacts(t *testing.T) {
	t.Parallel()

	collector := &Collector{procRoot: t.TempDir(), sysRoot: t.TempDir()}
	write(t, filepath.Join(collector.sysRoot, "class/dmi/id/product_name"), "KVM")
	collector.detectVirt = func(context.Context, string) (string, error) { return "", os.ErrNotExist }
	if facts := collector.guestFacts(t.Context(), nil); facts[series.FactCPUHypervisor] != "KVM" {
		t.Fatalf("failed detector erased sysfs results: %v", facts)
	}
	write(t, filepath.Join(collector.procRoot, "xen/capabilities"), "control_d")
	write(t, filepath.Join(collector.sysRoot, "hypervisor/type"), "xen")
	if facts := collector.guestFacts(t.Context(), []string{"hypervisor"}); len(facts) != 0 {
		t.Fatalf("Xen control domain was identified as a guest: %v", facts)
	}
}

func TestSystemdVirtualizationAcceptsOnlyTheDocumentedNoneExitStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "systemd-detect-virt")
	t.Setenv("PATH", dir)
	for _, test := range []struct {
		output string
		exit   string
		ok     bool
	}{{"kvm", "0", true}, {"none", "1", true}, {"none", "2", false}, {"", "1", false}} {
		write(t, path, "#!/bin/sh\nprintf '%s\\n' '"+test.output+"'\nexit "+test.exit+"\n")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := systemdVirtualization(t.Context(), "--vm")
		if (err == nil) != test.ok || strings.TrimSpace(got) != test.output {
			t.Fatalf("output=%q exit=%s: got %q, %v", test.output, test.exit, got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := systemdVirtualization(ctx, "--vm"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled detection returned %v", err)
	}
}
