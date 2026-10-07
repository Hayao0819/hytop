//go:build linux

// Package psi reads /proc/pressure.
package psi

import (
	"context"
	"time"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

var resources = []string{"cpu", "io", "memory"}

type Collector struct{ fs procfs.FS }

func New(root string) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}

	return &Collector{fs: fs}, nil
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.PSIStatsForResource("cpu")

	return collect.KernelAvailability(err, "build the kernel with CONFIG_PSI=y, or boot with psi=1")
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	var samples []metric.Sample

	for _, resource := range resources {
		stats, err := c.fs.PSIStatsForResource(resource)
		if err != nil {
			// cpu has no "full" line and a kernel may omit a resource entirely.
			continue
		}

		if stats.Some != nil {
			samples = appendPressure(samples, resource, "some", stats.Some, now)
		}

		if stats.Full != nil {
			samples = appendPressure(samples, resource, "full", stats.Full, now)
		}
	}

	if len(samples) == 0 {
		return nil, errors.New("no pressure information was readable")
	}

	return samples, nil
}

func appendPressure(samples []metric.Sample, resource, kind string, values *procfs.PSILine, now time.Time) []metric.Sample {
	return append(samples,
		metric.Sample{Key: key(resource, kind, "avg10"), Value: values.Avg10, Time: now},
		metric.Sample{Key: key(resource, kind, "avg60"), Value: values.Avg60, Time: now},
		metric.Sample{Key: key(resource, kind, "avg300"), Value: values.Avg300, Time: now},
	)
}

func key(resource, kind, window string) series.Key {
	return series.Key("psi." + resource + "." + kind + "." + window)
}
