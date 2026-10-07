//go:build linux

package diskio

import "testing"

func TestAPartitionIsNotCountedBesideItsWholeDevice(t *testing.T) {
	t.Parallel()

	whole := map[string]bool{
		"nvme0n1": true, "sda": true, "mmcblk0": true, "nbd0": true, "md0": true, "sr0": true,
	}

	for name, want := range map[string]bool{
		"nvme0n1":   true,
		"nvme0n1p2": false,
		"sda":       true,
		"sda1":      false,
		"mmcblk0":   true,
		"mmcblk0p1": false,
		"nbd0":      true,
		"nbd0p1":    false,
		"md0":       true,
		"md0p1":     false,
		"sr0":       true,
		"loop0":     false,
		"zram0":     false,
		"dm-0":      false,
	} {
		if got := interesting(name, whole); got != want {
			t.Errorf("interesting(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestWithoutSysfsTheNameRulesStandIn(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{
		"nvme0n1": true, "nvme0n1p2": false,
		"sda": true, "sda1": false,
		"vdb": true, "vdb2": false,
		"loop0": false, "zram0": false,
	} {
		if got := interesting(name, nil); got != want {
			t.Errorf("interesting(%q, nil) = %v, want %v", name, got, want)
		}
	}
}
