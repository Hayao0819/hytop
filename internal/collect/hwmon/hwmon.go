//go:build linux

// Package hwmon reads temperature, fan, and power sensors from sysfs.
package hwmon

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct{ root string }

func New(sysRoot string) *Collector {
	return &Collector{root: filepath.Join(sysRoot, "class", "hwmon")}
}

func (c *Collector) Check() collect.Availability {
	entries, err := os.ReadDir(c.root)
	if err != nil || len(entries) == 0 {
		return collect.Availability{
			State:  collect.NoHardware,
			Reason: "no hwmon devices",
			Remedy: "load the driver for this board's sensors, such as nct6775",
		}
	}

	return collect.Availability{State: collect.Ready}
}

// reading is one sensor file: what it measures and what to divide by.
type sensorReading struct {
	prefix  string
	suffix  string
	divisor float64
	kind    string
}

var readings = []sensorReading{
	{"temp", "_input", 1000, "thermal"},
	{"fan", "_input", 1, "fan"},
	{"power", "_average", 1e6, "power"},
	{"power", "_input", 1e6, "power"},
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	devices, err := os.ReadDir(c.root)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", c.root)
	}

	var (
		samples []metric.Sample
		hottest float64
		seen    bool
	)

	for _, device := range devices {
		dir := filepath.Join(c.root, device.Name())
		chip := readName(filepath.Join(dir, "name"), device.Name())

		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

		for _, file := range files {
			for _, reading := range readings {
				if !strings.HasPrefix(file.Name(), reading.prefix) || !strings.HasSuffix(file.Name(), reading.suffix) {
					continue
				}
				value, ok := reading.read(dir, file.Name())
				if !ok {
					continue
				}

				value /= reading.divisor

				label := labelOf(dir, file.Name(), reading.suffix)

				samples = append(samples, metric.Sample{
					Key:   key(reading.kind, chip, label),
					Value: value,
					Time:  now,
				})

				if reading.kind == "thermal" && value > 0 && value < 200 {
					hottest = max(hottest, value)
					seen = true
				}
			}
		}
	}

	if seen {
		samples = append(samples, metric.Sample{Key: "thermal.max.temp", Value: hottest, Time: now})
	}

	if len(samples) == 0 {
		return nil, errors.New("no sensor was readable")
	}

	return samples, nil
}

func (r sensorReading) read(dir, file string) (float64, bool) {
	if r.prefix == "power" && r.suffix == "_input" {
		channel := strings.TrimSuffix(file, r.suffix)
		if _, ok := sysread.String(filepath.Join(dir, channel+"_average")); ok {
			return 0, false
		}
	}

	return sysread.Float(filepath.Join(dir, file))
}

// labelOf prefers the name the driver gives a channel over its file number,
// since "Package id 0" says more than "temp1".
func labelOf(dir, file, suffix string) string {
	channel := strings.TrimSuffix(file, suffix)

	if name, ok := sysread.String(filepath.Join(dir, channel+"_label")); ok {
		return name
	}

	return channel
}

func key(kind, chip, label string) series.Key {
	what := map[string]string{"thermal": "temp", "fan": "rpm", "power": "watts"}[kind]

	return series.Key(kind + "." + clean(chip) + "_" + clean(label) + "." + what)
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, s)

	return strings.Trim(s, "_")
}

func readName(path, fallback string) string {
	if name, ok := sysread.String(path); ok {
		return name
	}

	return fallback
}
