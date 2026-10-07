//go:build !linux

// Package power reads batteries through native operating-system APIs.
package power

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/distatus/battery"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

type Collector struct {
	mu        sync.RWMutex
	batteries []*battery.Battery
}

func New(string) *Collector { return &Collector{} }

func (c *Collector) Check() collect.Availability {
	batteries, err := battery.GetAll()
	if len(batteries) == 0 {
		reason := "no battery was found"
		if err != nil {
			reason = err.Error()
		}

		return collect.Availability{State: collect.NoHardware, Reason: reason}
	}
	c.replace(batteries)

	return collect.Availability{State: collect.Ready}
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	batteries, err := battery.GetAll()
	c.replace(batteries)
	if len(batteries) == 0 {
		return nil, err
	}

	var samples []metric.Sample
	for index, value := range batteries {
		if value == nil {
			continue
		}
		prefix := fmt.Sprintf("battery.%d.", index)
		add := func(key string, reading float64) {
			if reading >= 0 {
				samples = append(samples, metric.Sample{Key: series.Key(prefix + key), Value: reading, Time: now})
			}
		}

		if value.Full > 0 {
			add("capacity", 100*value.Current/value.Full)
		}
		add("energy", value.Current/1000)
		add("energy_full", value.Full/1000)
		add("energy_design", value.Design/1000)
		add("power", value.ChargeRate/1000)
		add("voltage", value.Voltage)
		if value.Design > 0 {
			add("health", 100*value.Full/value.Design)
		}
		if value.ChargeRate > 0 {
			switch value.State.Raw {
			case battery.Discharging:
				add("time_to_empty", 3600*value.Current/value.ChargeRate)
			case battery.Charging:
				add("time_to_full", 3600*(value.Full-value.Current)/value.ChargeRate)
			}
		}
	}

	return samples, nil
}

func (c *Collector) Facts(context.Context) (collect.Facts, error) {
	c.mu.RLock()
	batteries := append([]*battery.Battery(nil), c.batteries...)
	c.mu.RUnlock()

	facts := make(collect.Facts)
	connection := ""
	for index, value := range batteries {
		if value == nil {
			continue
		}
		id := strconv.Itoa(index)
		facts["battery.name."+id] = "Battery " + id
		facts["battery.status."+id] = value.State.String()
		if index == 0 {
			facts[series.FactBatteryStatus] = value.State.String()
			if value.Design > 0 {
				facts[series.FactBatteryHealth] = strconv.FormatFloat(100*value.Full/value.Design, 'f', 0, 64) + " %"
			}
		}
		switch value.State.Raw {
		case battery.Charging, battery.Full, battery.Idle:
			connection = "connected"
		case battery.Discharging, battery.Empty:
			if connection == "" {
				connection = "on battery"
			}
		}
	}
	if connection != "" {
		facts[series.FactACOnline] = connection
	}

	return facts, nil
}

func (c *Collector) replace(batteries []*battery.Battery) {
	c.mu.Lock()
	c.batteries = append(c.batteries[:0], batteries...)
	c.mu.Unlock()
}

func (c *Collector) Batteries() []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indices := make([]int, 0, len(c.batteries))
	for index, value := range c.batteries {
		if value != nil {
			indices = append(indices, index)
		}
	}

	return indices
}
