//go:build linux

// Package power reads batteries and adapters from sysfs.
package power

import (
	"context"
	"maps"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/procfs/sysfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	fs      sysfs.FS
	sysRoot string
	err     error

	mu               sync.RWMutex
	batteryCount     int
	supplies         sysfs.PowerSupplyClass
	energy           map[string]batteryEnergyReading
	historyAttempted bool
}

type batteryEnergyReading struct {
	microWh int64
	time    time.Time
	power   float64
	valid   bool
}

func New(sysRoot string) *Collector {
	fs, err := sysfs.NewFS(sysRoot)

	return &Collector{fs: fs, sysRoot: sysRoot, err: err, energy: make(map[string]batteryEnergyReading)}
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
	batteries := byType(supplies, "Battery")
	if len(batteries)+len(byType(supplies, "Mains")) == 0 {
		return collect.Availability{State: collect.NoHardware, Reason: "no battery or mains supply"}
	}
	c.remember(supplies, len(batteries))

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

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	supplies, err := c.fs.PowerSupplyClass()
	if err != nil {
		return nil, errors.Wrap(err, "reading power supplies")
	}
	var samples []metric.Sample
	batteries := byType(supplies, "Battery")
	c.remember(supplies, len(batteries))

	if len(batteries) > 0 && c.takeHistoryAttempt() {
		samples = append(samples, loadUPowerHistory(ctx, batteries, now)...)
	}

	for n, battery := range batteries {
		id := strconv.Itoa(n)
		prefix := "battery." + id + "."
		add := func(key string, value float64) {
			samples = append(samples, metric.Sample{Key: series.Key(prefix + key), Value: value, Time: now})
		}

		if capacity, ok := batteryCapacity(battery); ok {
			add("capacity", capacity)
		}

		power, hasPower := c.batteryPower(battery, now)
		if hasPower {
			add("power", power)
		}

		addMicro(add, "energy", batteryEnergyNow(battery))
		addMicro(add, "energy_full", batteryEnergyFull(battery))
		addMicro(add, "energy_design", batteryEnergyDesign(battery))
		addMicro(add, "voltage", battery.VoltageNow)
		addInt(add, "cycles", battery.CycleCount)
		if remaining := first(battery.TimeToEmptyNow, battery.TimeToEmptyAvg); remaining != nil {
			addInt(add, "time_to_empty", remaining)
		} else if seconds, ok := estimatedTime(battery, false, power); ok {
			add("time_to_empty", seconds)
		}
		if remaining := first(battery.TimeToFullNow, battery.TimeToFullAvg); remaining != nil {
			addInt(add, "time_to_full", remaining)
		} else if seconds, ok := estimatedTime(battery, true, power); ok {
			add("time_to_full", seconds)
		}

		if battery.Temp != nil {
			add("temp", float64(*battery.Temp)/10)
		}
		if health, ok := batteryHealth(battery); ok {
			add("health", health)
		}

		dir := filepath.Join(c.sysRoot, "class", "power_supply", battery.Name)
		addFile(add, "charge_start", filepath.Join(dir, "charge_control_start_threshold"))
		addFile(add, "charge_end", filepath.Join(dir, "charge_control_end_threshold"))
	}

	if len(samples) == 0 && len(batteries) > 0 {
		return nil, errors.New("no battery reading was available")
	}

	return samples, nil
}

func (c *Collector) batteryPower(battery sysfs.PowerSupply, now time.Time) (float64, bool) {
	energy := batteryEnergyNow(battery)
	// A battery reports either microwatts directly or microamps at a voltage.
	var direct float64
	var hasDirect bool
	if reading := first(battery.PowerNow, battery.PowerAvg); reading != nil {
		direct, hasDirect = math.Abs(float64(*reading)/1e6), true
	} else if current, voltage := first(battery.CurrentNow, battery.CurrentAvg),
		first(battery.VoltageNow, battery.VoltageAvg); current != nil && voltage != nil {
		direct, hasDirect = math.Abs(float64(*current)*float64(*voltage)/1e12), true
	}
	if energy == nil {
		return direct, hasDirect
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	previous, hadPrevious := c.energy[battery.Name]
	if hasDirect {
		c.energy[battery.Name] = batteryEnergyReading{
			microWh: *energy, time: now, power: direct, valid: true,
		}

		return direct, true
	}
	if battery.Status != "" && battery.Status != "Charging" && battery.Status != "Discharging" {
		c.energy[battery.Name] = batteryEnergyReading{microWh: *energy, time: now, valid: true}

		return 0, true
	}
	if !hadPrevious {
		c.energy[battery.Name] = batteryEnergyReading{microWh: *energy, time: now}

		return 0, false
	}
	if *energy == previous.microWh {
		return previous.power, previous.valid
	}
	if !now.After(previous.time) {
		return 0, false
	}

	power := math.Abs(float64(*energy-previous.microWh)/1e6) / now.Sub(previous.time).Hours()
	c.energy[battery.Name] = batteryEnergyReading{
		microWh: *energy, time: now, power: power, valid: true,
	}

	return power, true
}

func (c *Collector) Facts(context.Context) (collect.Facts, error) {
	supplies := c.supplySnapshot()
	if supplies == nil {
		var err error
		supplies, err = c.fs.PowerSupplyClass()
		if err != nil {
			return nil, errors.Wrap(err, "reading power supplies")
		}
		c.remember(supplies, len(byType(supplies, "Battery")))
	}
	facts := collect.Facts{}

	if batteries := byType(supplies, "Battery"); len(batteries) > 0 {
		battery := batteries[0]
		if battery.Status != "" {
			facts[series.FactBatteryStatus] = battery.Status
		}

		if health, ok := batteryHealth(battery); ok {
			facts[series.FactBatteryHealth] = strconv.FormatFloat(health, 'f', 0, 64) + " %"
		}

		for n, battery := range batteries {
			id := strconv.Itoa(n)
			setFact(facts, "battery.name."+id, firstText(battery.ModelName, battery.Name))
			setFact(facts, "battery.status."+id, battery.Status)
			setFact(facts, "battery.vendor."+id, battery.Manufacturer)
			setFact(facts, "battery.model."+id, battery.ModelName)
			setFact(facts, "battery.serial."+id, battery.SerialNumber)
			setFact(facts, "battery.technology."+id, battery.Technology)
			setFact(facts, "battery.health_status."+id, battery.Health)
		}
	}

	for _, mains := range byType(supplies, "Mains") {
		if mains.Online != nil {
			switch *mains.Online {
			case 0:
				facts[series.FactACOnline] = "on battery"
			case 1:
				facts[series.FactACOnline] = "connected"
			}
		}
	}

	if len(facts) == 0 && len(supplies) > 0 {
		return nil, errors.New("no power information was readable")
	}

	return facts, nil
}

func (c *Collector) takeHistoryAttempt() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.historyAttempted || filepath.Clean(c.sysRoot) != "/sys" {
		return false
	}
	c.historyAttempted = true

	return true
}

func (c *Collector) remember(supplies sysfs.PowerSupplyClass, batteries int) {
	c.mu.Lock()
	c.batteryCount = batteries
	c.supplies = maps.Clone(supplies)
	c.mu.Unlock()
}

func (c *Collector) supplySnapshot() sysfs.PowerSupplyClass {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return maps.Clone(c.supplies)
}

// Batteries returns stable indices for the currently discovered batteries.
func (c *Collector) Batteries() []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indices := make([]int, c.batteryCount)
	for i := range indices {
		indices[i] = i
	}

	return indices
}

func addMicro(add func(string, float64), key string, value *int64) {
	if value != nil {
		add(key, float64(*value)/1e6)
	}
}

func addInt(add func(string, float64), key string, value *int64) {
	if value != nil && *value >= 0 {
		add(key, float64(*value))
	}
}

func addFile(add func(string, float64), key, path string) {
	if value, ok := sysread.Float(path); ok {
		add(key, value)
	}
}

func first(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func batteryEnergyNow(battery sysfs.PowerSupply) *int64 {
	return energy(battery.EnergyNow, battery.ChargeNow, battery.VoltageNow)
}

func batteryCapacity(battery sysfs.PowerSupply) (float64, bool) {
	if battery.Capacity != nil {
		return float64(*battery.Capacity), true
	}

	now, full := batteryEnergyNow(battery), batteryEnergyFull(battery)
	if now == nil || full == nil || *full <= 0 {
		return 0, false
	}

	return min(100, max(0, 100*float64(*now)/float64(*full))), true
}

func batteryEnergyFull(battery sysfs.PowerSupply) *int64 {
	return energy(battery.EnergyFull, battery.ChargeFull, battery.VoltageNow)
}

func batteryEnergyDesign(battery sysfs.PowerSupply) *int64 {
	return energy(battery.EnergyFullDesign, battery.ChargeFullDesign,
		first(battery.VoltageMinDesign, battery.VoltageMaxDesign, battery.VoltageNow))
}

func energy(direct, charge, voltage *int64) *int64 {
	if direct != nil {
		return direct
	}
	if charge == nil || voltage == nil {
		return nil
	}

	value := *charge * *voltage / 1e6

	return &value
}

func batteryHealth(battery sysfs.PowerSupply) (float64, bool) {
	full, design := batteryEnergyFull(battery), batteryEnergyDesign(battery)
	if full == nil || design == nil || *design <= 0 {
		return 0, false
	}

	return 100 * float64(*full) / float64(*design), true
}

func estimatedTime(battery sysfs.PowerSupply, charging bool, power float64) (float64, bool) {
	want := "Discharging"
	if charging {
		want = "Charging"
	}
	if battery.Status != want {
		return 0, false
	}

	if power <= 0 {
		return 0, false
	}

	now, full := batteryEnergyNow(battery), batteryEnergyFull(battery)
	if now == nil {
		return 0, false
	}
	energy := float64(*now) / 1e6
	if charging {
		if full == nil || *full <= *now {
			return 0, false
		}
		energy = float64(*full-*now) / 1e6
	}

	return 3600 * energy / power, true
}

func setFact(facts collect.Facts, key, value string) {
	if value != "" {
		facts[key] = value
	}
}

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
