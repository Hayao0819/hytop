//go:build linux

// Package net reads /proc/net/dev.
package net

import (
	"context"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/errors"
)

type reading struct {
	rx, tx uint64
	at     time.Time
}

type Collector struct {
	fs       procfs.FS
	sys      sysfs.FS
	previous map[string]reading
	mu       sync.RWMutex
	activity map[string]uint64
	links    sysfs.NetClass
	linksAt  time.Time
}

const linkRefresh = 5 * time.Second

func New(root, sysRoot string) (*Collector, error) {
	fs, err := procfs.NewFS(root)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", root)
	}
	sys, err := sysfs.NewFS(sysRoot)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", sysRoot)
	}

	return &Collector{
		fs: fs, sys: sys, previous: make(map[string]reading), activity: make(map[string]uint64),
	}, nil
}

// Interfaces returns linked or active interfaces ordered by traffic.
func (c *Collector) Interfaces() []string {
	devices, err := c.fs.NetDev()
	if err != nil {
		return nil
	}
	links := c.linkState(time.Now())

	c.mu.RLock()
	activity := maps.Clone(c.activity)
	c.mu.RUnlock()

	return interfaces(devices, links, activity)
}

func interfaces(devices procfs.NetDev, links sysfs.NetClass, activity map[string]uint64) []string {
	type card struct {
		name     string
		active   uint64
		carrier  bool
		lifetime uint64
	}

	cards := make([]card, 0, len(devices))

	for name, device := range devices {
		if name == "lo" {
			continue
		}

		traffic := device.RxBytes + device.TxBytes
		link := links[name]
		if traffic == 0 && (link.Carrier == nil || *link.Carrier != 1) {
			continue
		}

		cards = append(cards, card{
			name: name, active: activity[name],
			carrier: link.Carrier != nil && *link.Carrier == 1, lifetime: traffic,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		if cards[i].active != cards[j].active {
			return cards[i].active > cards[j].active
		}
		if cards[i].carrier != cards[j].carrier {
			return cards[i].carrier
		}
		if cards[i].lifetime != cards[j].lifetime {
			return cards[i].lifetime > cards[j].lifetime
		}

		return cards[i].name < cards[j].name
	})

	names := make([]string, 0, len(cards))
	for _, c := range cards {
		names = append(names, c.name)
	}

	return names
}

// speed reads the negotiated link rate. A down or virtual interface reports -1
// or refuses the read, and has no speed to show.
func speed(link sysfs.NetClassIface) (float64, bool) {
	if link.Speed == nil || *link.Speed <= 0 {
		return 0, false
	}

	return float64(*link.Speed) * 1e6, true
}

func (c *Collector) linkState(now time.Time) sysfs.NetClass {
	c.mu.Lock()
	defer c.mu.Unlock()

	age := now.Sub(c.linksAt)
	if c.links != nil && age >= 0 && age < linkRefresh {
		return c.links
	}

	links, err := c.sys.NetClass()
	if err == nil {
		c.links, c.linksAt = links, now
	}

	return c.links
}

func (c *Collector) Check() collect.Availability {
	_, err := c.fs.NetDev()

	return collect.KernelAvailability(err, "mount /proc")
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	devices, err := c.fs.NetDev()
	if err != nil {
		return nil, errors.Wrap(err, "reading /proc/net/dev")
	}
	links := c.linkState(now)

	var (
		samples                    []metric.Sample
		totalRx, totalTx           float64
		totalRxBytes, totalTxBytes float64
		hadHistory                 bool
	)

	// Limit per-interface history to active or linked interfaces.
	wanted := make(map[string]bool, len(devices))
	c.mu.RLock()
	activity := make(map[string]uint64, len(c.activity))
	for name, bytes := range c.activity {
		activity[name] = bytes
	}
	c.mu.RUnlock()
	for _, name := range interfaces(devices, links, activity) {
		wanted[name] = true
	}

	for name, device := range devices {
		if !wanted[name] {
			continue
		}

		current := reading{rx: device.RxBytes, tx: device.TxBytes, at: now}

		previous, seen := c.previous[name]
		c.previous[name] = current

		if !seen {
			continue
		}

		elapsed := current.at.Sub(previous.at).Seconds()
		if elapsed <= 0 {
			continue
		}
		if current.rx < previous.rx || current.tx < previous.tx {
			c.setActivity(name, 0)
			continue
		}

		hadHistory = true
		c.setActivity(name, current.rx-previous.rx+current.tx-previous.tx)

		var (
			rx = float64(current.rx-previous.rx) / elapsed
			tx = float64(current.tx-previous.tx) / elapsed
		)

		totalRx += rx
		totalTx += tx

		samples = append(samples,
			metric.Sample{Key: key(name, "rx"), Value: rx, Time: now},
			metric.Sample{Key: key(name, "tx"), Value: tx, Time: now},
			metric.Sample{Key: key(name, "rx.total"), Value: float64(current.rx), Time: now},
			metric.Sample{Key: key(name, "tx.total"), Value: float64(current.tx), Time: now},
		)

		if speed, ok := speed(links[name]); ok {
			samples = append(samples, metric.Sample{Key: key(name, "speed"), Value: speed, Time: now})
		}

		totalRxBytes += float64(current.rx)
		totalTxBytes += float64(current.tx)
	}

	for name := range c.previous {
		if _, exists := devices[name]; !exists {
			delete(c.previous, name)
			c.setActivity(name, 0)
		}
	}

	if hadHistory {
		samples = append(samples,
			metric.Sample{Key: "net.total.rx", Value: totalRx, Time: now},
			metric.Sample{Key: "net.total.tx", Value: totalTx, Time: now},
			metric.Sample{Key: "net.total.rx.total", Value: totalRxBytes, Time: now},
			metric.Sample{Key: "net.total.tx.total", Value: totalTxBytes, Time: now},
		)
	}

	return samples, nil
}

func (c *Collector) setActivity(name string, bytes uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if bytes == 0 {
		delete(c.activity, name)
		return
	}
	c.activity[name] = bytes
}
