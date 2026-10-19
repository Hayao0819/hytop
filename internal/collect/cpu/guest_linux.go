package cpu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
)

func (c *Collector) guestFacts(ctx context.Context, flags []string) collect.Facts {
	hypervisor := ""
	if c.detectVirt != nil {
		if value, err := c.detectVirt(ctx, "--vm"); err == nil {
			hypervisor = value
		}
	}
	if hypervisor == "" {
		hypervisor = c.guestFromSysfs(flags)
	}
	if hypervisor == "none" {
		hypervisor = ""
	}
	if name, ok := hypervisorNames[hypervisor]; ok {
		hypervisor = name
	}
	confidential := c.confidentialGuest(flags)
	if confidential == "" && c.detectVirt != nil {
		if value, err := c.detectVirt(ctx, "--cvm"); err == nil {
			confidential = confidentialNames[value]
		}
	}
	return guestFacts(hypervisor, confidential)
}

var hypervisorNames = map[string]string{
	"kvm": "KVM", "qemu": "QEMU", "microsoft": "Hyper-V", "vmware": "VMware",
	"oracle": "VirtualBox", "xen": "Xen", "parallels": "Parallels",
	"amazon": "Amazon EC2", "google": "Google Compute Engine", "apple": "Apple Virtualization",
	"other": "Unknown",
}

var confidentialNames = map[string]string{
	"tdx": "Intel TDX", "sev": "AMD SEV", "sev-es": "AMD SEV-ES", "sev-snp": "AMD SEV-SNP",
	"protvirt": "IBM Secure Execution", "cca": "Arm CCA",
}

func (c *Collector) guestFromSysfs(flags []string) string {
	if slices.Contains(strings.Fields(sysread.Text(filepath.Join(c.procRoot, "xen", "capabilities"))), "control_d") {
		return ""
	}
	if value := sysread.Text(filepath.Join(c.sysRoot, "hypervisor", "type")); value != "" {
		return value
	}
	vendor := sysread.Text(filepath.Join(c.sysRoot, "class", "dmi", "id", "sys_vendor"))
	product := sysread.Text(filepath.Join(c.sysRoot, "class", "dmi", "id", "product_name"))
	if value := dmiHypervisor(vendor, product); value != "" {
		return value
	}
	if slices.Contains(flags, "hypervisor") {
		return "Unknown"
	}
	return ""
}

func (c *Collector) confidentialGuest(flags []string) string {
	if slices.Contains(flags, "tdx_guest") {
		return "Intel TDX"
	}
	// These guest-only devices indicate active protection, unlike CPU capability flags.
	for _, device := range []struct{ path, name string }{
		{"class/misc/tdx_guest", "Intel TDX"},
		{"devices/platform/sev-guest", "AMD SEV-SNP"},
		{"devices/system/cpu/sev", "AMD SEV-SNP"},
	} {
		if info, err := os.Stat(filepath.Join(c.sysRoot, device.path)); err == nil && info.IsDir() {
			return device.name
		}
	}
	return ""
}

func systemdVirtualization(ctx context.Context, flag string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "systemd-detect-virt", flag).Output()
	value := strings.TrimSpace(string(output))
	if exited, ok := err.(*exec.ExitError); ok && exited.ExitCode() == 1 && value == "none" {
		return value, nil
	}
	return value, err
}
