//go:build linux

// Package units lists systemd units over D-Bus.
package units

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	systemddb "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	address string

	mu    sync.RWMutex
	units []unitmodel.Unit
}

func New(address string) *Collector { return &Collector{address: address} }

func (c *Collector) connect(ctx context.Context) (*systemddb.Conn, error) {
	if c.address == "" {
		return systemddb.NewSystemConnectionContext(ctx)
	}

	return systemddb.NewConnection(func() (*dbus.Conn, error) {
		conn, err := dbus.Dial(c.address, dbus.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		if err := conn.Auth(nil); err != nil {
			_ = conn.Close()

			return nil, err
		}

		if err := conn.Hello(); err != nil {
			_ = conn.Close()

			return nil, err
		}

		return conn, nil
	})
}

func (c *Collector) Check() collect.Availability {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := c.connect(ctx)
	if err != nil {
		return collect.Availability{
			State:  collect.NoKernelSupport,
			Reason: err.Error(),
			Remedy: "systemd's D-Bus socket is not reachable",
		}
	}

	defer conn.Close()

	if _, err := conn.ListUnitsContext(ctx); err != nil {
		return collect.Availability{State: collect.NoKernelSupport, Reason: err.Error()}
	}

	return collect.Availability{State: collect.Ready}
}

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to the system bus")
	}

	defer conn.Close()

	statuses, err := conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing units")
	}

	units := make([]unitmodel.Unit, 0, len(statuses))
	for _, status := range statuses {
		units = append(units, unitmodel.Unit{
			Name:        safe.Text(status.Name),
			Description: safe.Text(status.Description),
			Load:        status.LoadState,
			Active:      status.ActiveState,
			Sub:         status.SubState,
			Following:   status.Followed,
			Path:        string(status.Path),
		})
	}

	c.mu.Lock()
	c.units = units
	c.mu.Unlock()

	samples := summarize(units, now)

	if jobs, err := conn.ListJobsContext(ctx); err == nil {
		samples = append(samples, metric.Sample{Key: "units.jobs", Value: float64(len(jobs)), Time: now})
	}

	return samples, nil
}

func summarize(units []unitmodel.Unit, now time.Time) []metric.Sample {
	var failed, active, running int

	for _, unit := range units {
		if unit.Failed() {
			failed++
		}
		if unit.Active == "active" {
			active++
		}
		// Active oneshot units may have exited; running is a sub-state.
		if unit.Sub == "running" {
			running++
		}
	}

	kinds := map[string]int{}

	for _, unit := range units {
		if at := strings.LastIndex(unit.Name, "."); at >= 0 {
			kinds[unit.Name[at+1:]]++
		}
	}

	samples := []metric.Sample{
		{Key: "units.total", Value: float64(len(units)), Time: now},
		{Key: "units.failed", Value: float64(failed), Time: now},
		{Key: "units.active", Value: float64(active), Time: now},
		{Key: "units.running", Value: float64(running), Time: now},
	}

	for _, kind := range []string{"service", "timer", "socket", "mount"} {
		samples = append(samples,
			metric.Sample{Key: series.Key("units." + kind), Value: float64(kinds[kind]), Time: now})
	}

	return samples
}

// Facts returns manager-level systemd state and boot timing.
func (c *Collector) Facts(ctx context.Context) (collect.Facts, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to the system bus")
	}

	defer conn.Close()

	facts := collect.Facts{}

	if state, err := managerValue(conn, "SystemState"); err == nil {
		if value, ok := state.(string); ok {
			facts[series.FactUnitsState] = value
		}
	}

	// The two timestamps are microseconds since boot, so their difference is
	// how long userspace took to come up — what systemd-analyze reports.
	started, startErr := managerValue(conn, "UserspaceTimestampMonotonic")
	finished, finishErr := managerValue(conn, "FinishTimestampMonotonic")

	if startErr == nil && finishErr == nil {
		from, fromOK := started.(uint64)
		to, toOK := finished.(uint64)

		if fromOK && toOK && to > from {
			took := time.Duration(to-from) * time.Microsecond
			facts[series.FactUnitsBoot] = took.Round(time.Millisecond).String()
		}
	}

	if len(facts) == 0 {
		return nil, errors.New("systemd told us nothing about itself")
	}

	return facts, nil
}

func managerValue(conn *systemddb.Conn, name string) (any, error) {
	text, err := conn.GetManagerProperty(name)
	if err != nil {
		return nil, errors.Wrapf(err, "reading systemd's %s", name)
	}

	empty, _ := dbus.ParseSignature("")
	value, err := dbus.ParseVariant(text, empty)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding systemd's %s", name)
	}

	return value.Value(), nil
}

// Units returns the latest listing; callers choose its presentation order.
func (c *Collector) Units() []unitmodel.Unit {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.units)
}
