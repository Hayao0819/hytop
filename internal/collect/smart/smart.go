//go:build linux

// Package smart combines sysfs metadata with SMART ioctl data.
package smart

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

// Reader maintains the latest drive snapshot.
type Reader struct {
	sysRoot string
	devRoot string

	mu       sync.Mutex
	devices  []diskmodel.Device
	elevated map[string]diskmodel.Device
}

func NewReader(sysRoot, devRoot string) *Reader {
	return &Reader{sysRoot: sysRoot, devRoot: devRoot, elevated: make(map[string]diskmodel.Device)}
}

// Devices returns the latest snapshot without accessing hardware.
func (r *Reader) Devices() []diskmodel.Device {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.devices)
}

// Refresh reads and publishes all drives.
func (r *Reader) Refresh(ctx context.Context) {
	devices, err := r.Read(ctx)
	if err != nil {
		return
	}

	r.publish(devices)
}

// Read returns one complete drive snapshot.
func (r *Reader) Read(ctx context.Context) ([]diskmodel.Device, error) {
	names, err := r.blockDevices()
	if err != nil {
		return nil, err
	}

	devices := make([]diskmodel.Device, 0, len(names))

	for _, name := range names {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		device := r.fromSysfs(name)
		r.fromDrive(&device)
		devices = append(devices, device)
	}

	return devices, nil
}

// ReplaceElevated publishes and caches a privileged snapshot.
func (r *Reader) ReplaceElevated(devices []diskmodel.Device) {
	r.mu.Lock()
	r.elevated = make(map[string]diskmodel.Device, len(devices))
	for _, device := range devices {
		if device.Reason == "" && device.Health != diskmodel.Unknown {
			r.elevated[device.Name] = device
		}
	}
	r.devices = slices.Clone(devices)
	r.mu.Unlock()
}

func (r *Reader) publish(devices []diskmodel.Device) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range devices {
		cached, ok := r.elevated[devices[i].Name]
		if ok && sameDrive(devices[i], cached) && strings.Contains(strings.ToLower(devices[i].Reason), "privilege") {
			keepSMART(&devices[i], cached)
		}
	}
	r.devices = slices.Clone(devices)
}

func sameDrive(current, cached diskmodel.Device) bool {
	if current.Serial != "" && cached.Serial != "" {
		return current.Serial == cached.Serial
	}

	return current.Model == cached.Model && current.Size == cached.Size
}

func keepSMART(current *diskmodel.Device, cached diskmodel.Device) {
	current.Reason = ""
	current.Health = cached.Health
	current.Temperature = cached.Temperature
	current.PowerOnTime = cached.PowerOnTime
	current.PowerCycles = cached.PowerCycles
	current.Wear = cached.Wear
	current.Written = cached.Written
	current.Read = cached.Read
	current.Attributes = slices.Clone(cached.Attributes)
}

// blockDevices returns physical whole-drive nodes.
func (r *Reader) blockDevices() ([]string, error) {
	root := filepath.Join(r.sysRoot, "block")

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, errors.Wrapf(err, "listing %s", root)
	}

	var names []string

	for _, entry := range entries {
		name := entry.Name()

		switch {
		case strings.HasPrefix(name, "loop"),
			strings.HasPrefix(name, "ram"),
			strings.HasPrefix(name, "zram"),
			strings.HasPrefix(name, "dm-"),
			strings.HasPrefix(name, "md"):
			continue
		}

		if _, err := os.Stat(filepath.Join(root, name, "device")); err != nil {
			continue
		}

		names = append(names, name)
	}

	sort.Strings(names)

	return names, nil
}

func (r *Reader) fromSysfs(name string) diskmodel.Device {
	base := filepath.Join(r.sysRoot, "block", name)

	device := diskmodel.Device{
		Name:     name,
		Model:    sysread.Text(filepath.Join(base, "device", "model")),
		Vendor:   sysread.Text(filepath.Join(base, "device", "vendor")),
		Serial:   sysread.Text(filepath.Join(base, "device", "serial")),
		Firmware: sysread.Text(filepath.Join(base, "device", "rev")),
		Sched:    scheduler(filepath.Join(base, "queue", "scheduler")),
		Wear:     -1,
	}

	// NVMe class devices expose their model one level deeper.
	if device.Model == "" {
		device.Model = sysread.Text(filepath.Join(base, "device", "device", "model"))
	}

	if sectors, err := strconv.ParseUint(sysread.Text(filepath.Join(base, "size")), 10, 64); err == nil {
		device.Size = sectors * 512
	}

	device.Rotating = sysread.Text(filepath.Join(base, "queue", "rotational")) == "1"

	switch {
	case strings.HasPrefix(name, "nvme"):
		device.Bus = "NVMe"
	case strings.HasPrefix(name, "mmcblk"):
		device.Bus = "eMMC/SD"
	case strings.HasPrefix(name, "sr"):
		device.Bus = "ATAPI"
	default:
		device.Bus = "SATA/SAS"
	}

	device.Optical = strings.HasPrefix(name, "sr")

	return device
}

// scheduler picks the chosen one out of "[none] mq-deadline kyber".
func scheduler(path string) string {
	for _, word := range strings.Fields(sysread.Text(path)) {
		if strings.HasPrefix(word, "[") {
			return strings.Trim(word, "[]")
		}
	}

	return ""
}
