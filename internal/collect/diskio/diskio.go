//go:build linux

// Package diskio reads /proc/diskstats.
package diskio

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/procfs/blockdevice"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

// Linux diskstats sectors are always 512 bytes.
const sectorSize = 512

type reading struct {
	read, write, ioTime uint64
	at                  time.Time
}

type Collector struct {
	fs       blockdevice.FS
	previous map[string]reading
}

func New(procRoot, sysRoot string) (*Collector, error) {
	fs, err := blockdevice.NewFS(procRoot, sysRoot)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s and %s", procRoot, sysRoot)
	}

	return &Collector{fs: fs, previous: make(map[string]reading)}, nil
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.ProcDiskstats()

	return collect.KernelAvailability(err, "mount /proc")
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	stats, err := c.fs.ProcDiskstats()
	if err != nil {
		return nil, errors.Wrap(err, "reading /proc/diskstats")
	}

	whole := c.whole()

	var (
		samples             []metric.Sample
		totalRd, totalWr    float64
		anyDeviceHadHistory bool
	)

	for _, stat := range stats {
		if !interesting(stat.DeviceName, whole) {
			continue
		}

		current := reading{
			read:   stat.ReadSectors * sectorSize,
			write:  stat.WriteSectors * sectorSize,
			ioTime: stat.IOsTotalTicks,
			at:     now,
		}

		previous, seen := c.previous[stat.DeviceName]
		c.previous[stat.DeviceName] = current

		if !seen {
			continue
		}

		elapsed := current.at.Sub(previous.at).Seconds()
		if elapsed <= 0 {
			continue
		}

		// Counter regression indicates reset or device replacement.
		if current.read < previous.read || current.write < previous.write ||
			current.ioTime < previous.ioTime {
			continue
		}

		anyDeviceHadHistory = true

		var (
			rd   = float64(current.read-previous.read) / elapsed
			wr   = float64(current.write-previous.write) / elapsed
			util = float64(current.ioTime-previous.ioTime) / (elapsed * 10)
		)

		totalRd += rd
		totalWr += wr

		samples = append(samples,
			metric.Sample{Key: key(stat.DeviceName, "read"), Value: rd, Time: now},
			metric.Sample{Key: key(stat.DeviceName, "write"), Value: wr, Time: now},
			metric.Sample{Key: key(stat.DeviceName, "util"), Value: min(util, 100), Time: now},
		)
	}

	if anyDeviceHadHistory {
		samples = append(samples,
			metric.Sample{Key: "diskio.total.read", Value: totalRd, Time: now},
			metric.Sample{Key: "diskio.total.write", Value: totalWr, Time: now},
		)
	}

	return samples, nil
}

func key(device, what string) series.Key {
	return series.Key("diskio." + series.NormalizeSegment(device) + "." + what)
}

// interesting uses /sys/block as the authoritative whole-device list to avoid
// double-counting partitions with device-specific naming schemes.
func interesting(name string, whole map[string]bool) bool {
	switch {
	case strings.HasPrefix(name, "loop"), strings.HasPrefix(name, "ram"),
		strings.HasPrefix(name, "zram"), strings.HasPrefix(name, "dm-"):
		return false
	}

	if whole == nil {
		return byName(name)
	}

	return whole[name]
}

func (c *Collector) whole() map[string]bool {
	names, err := c.fs.SysBlockDevices()
	if err != nil {
		return nil
	}

	devices := make(map[string]bool, len(names))
	for _, name := range names {
		devices[name] = true
	}

	return devices
}

// byName is a conservative fallback when /sys/block is unavailable.
func byName(name string) bool {
	last := name[len(name)-1]
	if last < '0' || last > '9' {
		return true
	}

	if strings.HasPrefix(name, "nvme") && strings.LastIndexByte(name, 'p') > 0 {
		return false
	}

	return !strings.HasPrefix(name, "sd") && !strings.HasPrefix(name, "hd") && !strings.HasPrefix(name, "vd")
}
