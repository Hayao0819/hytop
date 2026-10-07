package cmd

import (
	"os"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/cpu"
	"github.com/Hayao0819/hytop/internal/collect/diskio"
	"github.com/Hayao0819/hytop/internal/collect/dmi"
	"github.com/Hayao0819/hytop/internal/collect/filesystem"
	"github.com/Hayao0819/hytop/internal/collect/gpu"
	"github.com/Hayao0819/hytop/internal/collect/hwmon"
	"github.com/Hayao0819/hytop/internal/collect/mem"
	"github.com/Hayao0819/hytop/internal/collect/net"
	"github.com/Hayao0819/hytop/internal/collect/power"
	"github.com/Hayao0819/hytop/internal/collect/proc"
	"github.com/Hayao0819/hytop/internal/collect/psi"
	"github.com/Hayao0819/hytop/internal/collect/units"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

func register(
	scheduler *collect.Scheduler, settings func() conf.Config, opts options,
) (*series.Registry, *devices, error) {
	type built struct {
		collector collect.Collector
		interval  func() time.Duration
	}

	var (
		all []built
		// Resolve intervals on each tick so hot reload affects active loops.
		base = func() time.Duration { return time.Duration(settings().General.Interval) }
		add  = func(c collect.Collector, err error, interval func() time.Duration) error {
			if err != nil {
				return err
			}

			all = append(all, built{collector: c, interval: interval})

			return nil
		}
	)

	registry, err := seriesRegistry()
	if err != nil {
		return nil, nil, err
	}

	cpuCollector, err := cpu.New(opts.procRoot, opts.sysRoot)
	if err := add(cpuCollector, err, base); err != nil {
		return nil, nil, err
	}

	memCollector, err := mem.New(opts.procRoot, opts.sysRoot)
	if err := add(memCollector, err, base); err != nil {
		return nil, nil, err
	}

	psiCollector, err := psi.New(opts.procRoot)
	if err := add(psiCollector, err, base); err != nil {
		return nil, nil, err
	}

	diskCollector, err := diskio.New(opts.procRoot, opts.sysRoot)
	if err := add(diskCollector, err, base); err != nil {
		return nil, nil, err
	}

	netCollector, err := net.New(opts.procRoot, opts.sysRoot)
	if err := add(netCollector, err, base); err != nil {
		return nil, nil, err
	}

	procCollector, err := proc.New(opts.procRoot, os.Getpid())
	if err := add(procCollector, err, base); err != nil {
		return nil, nil, err
	}

	// Sensors and batteries move slowly and cost a syscall each, so they are
	// read at a fifth of the rate the counters are.
	slow := func() time.Duration { return max(base()*5, 2*time.Second) }

	// The board's memory slots do not change while hytop is running; this is
	// only re-read in case udev filled the database in late.
	hourly := func() time.Duration { return time.Hour }

	gpuCollector := gpu.New(opts.sysRoot, opts.runRoot)
	if err := add(gpuCollector, nil, base); err != nil {
		return nil, nil, err
	}

	if err := add(hwmon.New(opts.sysRoot), nil, slow); err != nil {
		return nil, nil, err
	}

	powerCollector := power.New(opts.sysRoot)
	if err := add(powerCollector, nil, slow); err != nil {
		return nil, nil, err
	}

	if err := add(dmi.New(opts.runRoot, opts.sysRoot), nil, hourly); err != nil {
		return nil, nil, err
	}

	unitCollector := units.New("")
	if err := add(unitCollector, nil, slow); err != nil {
		return nil, nil, err
	}

	fsCollector := filesystem.New(opts.procRoot)
	if err := add(fsCollector, nil, slow); err != nil {
		return nil, nil, err
	}

	for _, b := range all {
		scheduler.Add(b.collector, b.interval)
	}

	return registry, &devices{
		interfaces: netCollector.Interfaces,
		gpus:       gpuCollector.Cards,
		batteries:  powerCollector.Batteries,
		units:      unitCollector.Units,
		mounts:     fsCollector.Mounts,
	}, nil
}

func seriesRegistry() (*series.Registry, error) {
	groups := []struct {
		name string
		defs []series.Def
	}{
		{"cpu", cpu.Defs},
		{"memory", mem.Defs},
		{"pressure", psi.Defs},
		{"disk I/O", diskio.Defs},
		{"network", net.Defs},
		{"process", proc.Defs},
		{"GPU", gpu.Defs},
		{"hardware monitor", hwmon.Defs},
		{"power", power.Defs},
		{"units", units.Defs},
		{"filesystem", filesystem.Defs},
	}

	registry := &series.Registry{}
	for _, group := range groups {
		if err := registry.Register(group.defs...); err != nil {
			return nil, errors.Wrapf(err, "registering %s series", group.name)
		}
	}

	return registry, nil
}
