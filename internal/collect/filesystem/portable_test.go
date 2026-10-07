//go:build !linux

package filesystem

import (
	"testing"

	psdisk "github.com/shirou/gopsutil/v4/disk"
)

func TestMountFromStatsKeepsReservedSpaceSeparate(t *testing.T) {
	t.Parallel()

	mount := mountFromStats(
		psdisk.PartitionStat{Device: "/dev/example", Mountpoint: "/data", Fstype: "example", Opts: []string{"rw"}},
		&psdisk.UsageStat{Total: 1000, Used: 600, Free: 300},
	)

	if mount.Free != 400 {
		t.Errorf("Free = %d, want 400", mount.Free)
	}
	if mount.Available != 300 {
		t.Errorf("Available = %d, want 300", mount.Available)
	}
	if got := mount.Usage(); got != 600.0/900.0*100 {
		t.Errorf("Usage() = %v, want %v", got, 600.0/900.0*100)
	}
}
