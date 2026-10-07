//go:build linux

package cpu

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

func (c *Collector) Facts(context.Context) (collect.Facts, error) {
	info, err := c.fs.CPUInfo()
	if err != nil {
		return nil, errors.Wrap(err, "reading /proc/cpuinfo")
	}

	if len(info) == 0 {
		return nil, errors.New("/proc/cpuinfo listed no processors")
	}

	facts := collect.Facts{
		series.FactCPUModel:   strings.TrimSpace(info[0].ModelName),
		series.FactCPUVendor:  info[0].VendorID,
		series.FactCPULogical: strconv.Itoa(len(info)),
	}

	sockets, cores := topology(info)
	facts[series.FactCPUSockets] = strconv.Itoa(sockets)
	facts[series.FactCPUCores] = strconv.Itoa(cores)

	facts[series.FactCPUVirtualisation] = virtualisation(info[0].Flags)

	if guest := c.guest(); guest != "" {
		facts[series.FactCPUHypervisor] = guest
	}

	if base, ok := c.baseSpeed(); ok {
		facts[series.FactCPUBaseSpeed] = base
	}

	for key, value := range c.caches() {
		facts[key] = value
	}

	return facts, nil
}

func topology(info []procfs.CPUInfo) (sockets, cores int) {
	var (
		physical = map[string]bool{}
		unique   = map[string]bool{}
	)

	for _, cpu := range info {
		physical[cpu.PhysicalID] = true
		unique[cpu.PhysicalID+"/"+cpu.CoreID] = true
	}

	return max(len(physical), 1), max(len(unique), 1)
}

// virtualisation reports the CPU's hardware virtualization extension.
func virtualisation(flags []string) string {
	for _, flag := range flags {
		switch flag {
		case "vmx":
			return "VT-x"
		case "svm":
			return "AMD-V"
		}
	}

	return "unavailable"
}

// guest reports the hypervisor this machine is itself running under, which is a
// different question from whether it can host one.
func (c *Collector) guest() string {
	if guest := sysread.Text(filepath.Join(c.sysRoot, "hypervisor", "type")); guest != "" {
		return guest
	}

	name := sysread.Text(filepath.Join(c.sysRoot, "class", "dmi", "id", "product_name"))
	if name == "" {
		return ""
	}

	for _, known := range []string{"KVM", "VMware", "VirtualBox", "QEMU", "Hyper-V", "Xen"} {
		if strings.Contains(name, known) {
			return known
		}
	}

	return ""
}

// baseSpeed prefers what cpufreq calls the base frequency, and falls back to
// the maximum the governor will reach.
func (c *Collector) baseSpeed() (string, bool) {
	for _, name := range []string{"base_frequency", "cpuinfo_max_freq"} {
		kHz, ok := sysread.Float(filepath.Join(c.sysRoot, "devices", "system", "cpu", "cpu0", "cpufreq", name))
		if !ok {
			continue
		}

		return strconv.FormatFloat(kHz/1e6, 'f', 2, 64) + " GHz", true
	}

	return "", false
}

var cacheFacts = map[string]string{
	"1Data":        series.FactCPUL1d,
	"1Instruction": series.FactCPUL1i,
	"2Unified":     series.FactCPUL2,
	"3Unified":     series.FactCPUL3,
}

// caches sums each level once per set of CPUs that share it.
func (c *Collector) caches() map[string]string {
	root := filepath.Join(c.sysRoot, "devices", "system", "cpu")

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	// A cache is shared, so it is counted once per set of CPUs that share it.
	seen := map[string]bool{}
	total := map[string]int64{}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "cpu") || !isNumber(entry.Name()[3:]) {
			continue
		}

		indexes, err := os.ReadDir(filepath.Join(root, entry.Name(), "cache"))
		if err != nil {
			continue
		}

		for _, index := range indexes {
			dir := filepath.Join(root, entry.Name(), "cache", index.Name())

			var (
				level  = sysread.Text(filepath.Join(dir, "level"))
				kind   = sysread.Text(filepath.Join(dir, "type"))
				shared = sysread.Text(filepath.Join(dir, "shared_cpu_list"))
				size   = sysread.Text(filepath.Join(dir, "size"))
			)

			fact, ok := cacheFacts[level+kind]
			if !ok || seen[fact+shared] {
				continue
			}

			seen[fact+shared] = true
			total[fact] += parseSize(size)
		}
	}

	facts := make(map[string]string, len(total))

	for fact, bytes := range total {
		facts[fact] = humanBytes(bytes)
	}

	return facts
}

// parseSize reads the "512K" and "32M" that sysfs writes cache sizes in.
func parseSize(s string) int64 {
	if s == "" {
		return 0
	}

	multiplier := int64(1)

	switch s[len(s)-1] {
	case 'K':
		multiplier, s = 1<<10, s[:len(s)-1]
	case 'M':
		multiplier, s = 1<<20, s[:len(s)-1]
	case 'G':
		multiplier, s = 1<<30, s[:len(s)-1]
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return n * multiplier
}

func humanBytes(n int64) string {
	units := []struct {
		suffix string
		size   int64
	}{{"MiB", 1 << 20}, {"KiB", 1 << 10}}

	for _, unit := range units {
		if n >= unit.size {
			return strconv.FormatFloat(float64(n)/float64(unit.size), 'f', -1, 64) + " " + unit.suffix
		}
	}

	return strconv.FormatInt(n, 10) + " B"
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
