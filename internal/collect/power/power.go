//go:build linux

// Package power reads batteries and adapters from sysfs.
package power

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/prometheus/procfs/sysfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	fs  sysfs.FS
	err error
}

func New(sysRoot string) *Collector {
	fs, err := sysfs.NewFS(sysRoot)

	return &Collector{fs: fs, err: err}
}

func (c *Collector) Check() collect.Availability {
	if c.err != nil {
		return collect.Availability{
			State:  collect.NoHardware,
			Reason: c.err.Error(),
		}
	}

	supplies, err := c.fs.PowerSupplyClass()
	if err != nil {
		return collect.Availability{State: collect.NoHardware, Reason: err.Error()}
	}
	if len(byType(supplies, "Battery"))+len(byType(supplies, "Mains")) == 0 {
		return collect.Availability{State: collect.NoHardware, Reason: "no battery or mains supply"}
	}

	return collect.Availability{State: collect.Ready}
}

func byType(supplies sysfs.PowerSupplyClass, kind string) []sysfs.PowerSupply {
	found := make([]sysfs.PowerSupply, 0, len(supplies))
	for _, supply := range supplies {
		if supply.Type == kind {
			found = append(found, supply)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })

	return found
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	supplies, err := c.fs.PowerSupplyClass()
	if err != nil {
		return nil, errors.Wrap(err, "reading power supplies")
	}
	var samples []metric.Sample
	batteries := byType(supplies, "Battery")

	for n, battery := range batteries {
		id := strconv.Itoa(n)

		if battery.Capacity != nil {
			samples = append(samples, metric.Sample{
				Key: series.Key("battery." + id + ".capacity"), Value: float64(*battery.Capacity), Time: now,
			})
		}

		// A battery reports either microwatts directly or microamps at a voltage.
		if battery.PowerNow != nil {
			samples = append(samples, metric.Sample{
				Key: series.Key("battery." + id + ".power"), Value: float64(*battery.PowerNow) / 1e6, Time: now,
			})
		} else if battery.CurrentNow != nil && battery.VoltageNow != nil {
			samples = append(samples, metric.Sample{
				Key:   series.Key("battery." + id + ".power"),
				Value: float64(*battery.CurrentNow) * float64(*battery.VoltageNow) / 1e12,
				Time:  now,
			})
		}
	}

	if len(samples) == 0 && len(batteries) > 0 {
		return nil, errors.New("no battery reading was available")
	}

	return samples, nil
}

func (c *Collector) Facts(context.Context) (collect.Facts, error) {
	supplies, err := c.fs.PowerSupplyClass()
	if err != nil {
		return nil, errors.Wrap(err, "reading power supplies")
	}
	facts := collect.Facts{}

	if batteries := byType(supplies, "Battery"); len(batteries) > 0 {
		battery := batteries[0]
		if battery.Status != "" {
			facts[series.FactBatteryStatus] = battery.Status
		}

		if battery.EnergyFull != nil && battery.EnergyFullDesign != nil && *battery.EnergyFullDesign > 0 {
			health := 100 * float64(*battery.EnergyFull) / float64(*battery.EnergyFullDesign)
			facts[series.FactBatteryHealth] = strconv.FormatFloat(health, 'f', 0, 64) + " %"
		}

	}

	for _, mains := range byType(supplies, "Mains") {
		if mains.Online != nil {
			facts[series.FactACOnline] = map[int64]string{1: "connected", 0: "on battery"}[*mains.Online]
		}
	}

	if len(facts) == 0 && len(supplies) > 0 {
		return nil, errors.New("no power information was readable")
	}

	return facts, nil
}
