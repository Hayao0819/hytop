//go:build !linux

package net

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	psnet "github.com/shirou/gopsutil/v4/net"
)

type reading struct {
	rx, tx uint64
	at     time.Time
}
type Collector struct {
	mu         sync.Mutex
	last       map[string]reading
	interfaces []string
	facts      collect.Facts
	factsAt    time.Time
}

func New(_ string, _ string) (*Collector, error) { return &Collector{last: map[string]reading{}}, nil }

func (c *Collector) Interfaces() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.interfaces)
}
func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }
func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	stats, err := psnet.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var samples []metric.Sample
	var totalRX, totalTX uint64
	var rateRX, rateTX float64
	list := make([]interfaceTraffic, 0, len(stats))
	for _, stat := range stats {
		totalRX += stat.BytesRecv
		totalTX += stat.BytesSent
		traffic := interfaceTraffic{name: stat.Name, lifetime: stat.BytesRecv + stat.BytesSent}
		samples = append(samples,
			metric.Sample{Key: key(stat.Name, "rx.total"), Value: float64(stat.BytesRecv), Time: now},
			metric.Sample{Key: key(stat.Name, "tx.total"), Value: float64(stat.BytesSent), Time: now},
		)
		current := reading{stat.BytesRecv, stat.BytesSent, now}
		previous, ok := c.last[stat.Name]
		c.last[stat.Name] = current
		if !ok {
			list = append(list, traffic)
			continue
		}
		seconds := now.Sub(previous.at).Seconds()
		if seconds <= 0 || stat.BytesRecv < previous.rx || stat.BytesSent < previous.tx {
			list = append(list, traffic)
			continue
		}
		traffic.active = stat.BytesRecv - previous.rx + stat.BytesSent - previous.tx
		list = append(list, traffic)
		rx := float64(stat.BytesRecv-previous.rx) / seconds
		tx := float64(stat.BytesSent-previous.tx) / seconds
		rateRX += rx
		rateTX += tx
		samples = append(samples,
			metric.Sample{Key: key(stat.Name, "rx"), Value: rx, Time: now},
			metric.Sample{Key: key(stat.Name, "tx"), Value: tx, Time: now},
		)
	}
	c.interfaces = rankInterfaces(list)
	samples = append(samples, metric.Sample{Key: "net.total.rx", Value: rateRX, Time: now}, metric.Sample{Key: "net.total.tx", Value: rateTX, Time: now}, metric.Sample{Key: "net.total.rx.total", Value: float64(totalRX), Time: now}, metric.Sample{Key: "net.total.tx.total", Value: float64(totalTX), Time: now})
	return samples, nil
}

func (c *Collector) Facts(ctx context.Context) (collect.Facts, error) {
	now := time.Now()
	c.mu.Lock()
	if c.facts != nil && now.Sub(c.factsAt) >= 0 && now.Sub(c.factsAt) < 5*time.Second {
		facts := maps.Clone(c.facts)
		c.mu.Unlock()

		return facts, nil
	}
	c.mu.Unlock()

	interfaces, err := psnet.InterfacesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	facts := make(collect.Facts)
	for _, iface := range interfaces {
		set := func(attribute, value string) {
			if value != "" {
				facts[fact(iface.Name, attribute)] = value
			}
		}
		addresses := make([]string, 0, len(iface.Addrs))
		for _, address := range iface.Addrs {
			addresses = append(addresses, address.Addr)
		}
		set("state", strings.Join(iface.Flags, ", "))
		set("address", iface.HardwareAddr)
		set("ip", strings.Join(addresses, ", "))
		if iface.MTU > 0 {
			set("mtu", strconv.Itoa(iface.MTU))
		}
	}
	c.mu.Lock()
	c.facts, c.factsAt = maps.Clone(facts), now
	c.mu.Unlock()

	return facts, nil
}
