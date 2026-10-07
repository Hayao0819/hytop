//go:build linux

package power

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/prometheus/procfs/sysfs"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

const (
	upowerService = "org.freedesktop.UPower"
	upowerPath    = dbus.ObjectPath("/org/freedesktop/UPower")
	upowerDevice  = "org.freedesktop.UPower.Device"
	week          = 7 * 24 * time.Hour
)

type historyItem struct {
	Time  uint32
	Value float64
	State uint32
}

func loadUPowerHistory(ctx context.Context, batteries []sysfs.PowerSupply, now time.Time) []metric.Sample {
	if len(batteries) == 0 {
		return nil
	}

	conn, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer func() { _ = conn.Close() }()

	var paths []dbus.ObjectPath
	if err := conn.Object(upowerService, upowerPath).
		CallWithContext(ctx, upowerService+".EnumerateDevices", 0).
		Store(&paths); err != nil {
		return nil
	}

	indices := make(map[string]int, len(batteries))
	for i, battery := range batteries {
		indices[battery.Name] = i
	}

	var samples []metric.Sample
	for _, path := range paths {
		object := conn.Object(upowerService, path)
		var properties map[string]dbus.Variant
		if err := object.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, upowerDevice).
			Store(&properties); err != nil {
			continue
		}

		property, ok := properties["NativePath"]
		if !ok {
			continue
		}
		native, ok := property.Value().(string)
		if !ok {
			continue
		}
		index, ok := indices[filepath.Base(native)]
		if !ok {
			continue
		}

		var history []historyItem
		if err := object.CallWithContext(ctx, upowerDevice+".GetHistory", 0,
			"charge", uint32(week/time.Second), uint32(2016)).Store(&history); err != nil {
			continue
		}

		samples = append(samples, historySamples(index, history, now)...)
	}

	return samples
}

func historySamples(index int, history []historyItem, now time.Time) []metric.Sample {
	slices.SortFunc(history, func(a, b historyItem) int { return cmp.Compare(a.Time, b.Time) })

	cutoff := now.Add(-week)
	key := series.Key("battery." + strconv.Itoa(index) + ".capacity")
	samples := make([]metric.Sample, 0, len(history))
	for _, item := range history {
		at := time.Unix(int64(item.Time), 0)
		if at.Before(cutoff) || at.After(now) || item.Value < 0 || item.Value > 100 {
			continue
		}
		samples = append(samples, metric.Sample{Key: key, Value: item.Value, Time: at})
	}

	return samples
}
