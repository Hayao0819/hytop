//go:build linux

package gpu

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

const (
	nvidiaQuery    = "pci.bus_id,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,clocks.gr"
	nvidiaInterval = 2 * time.Second
)

// collectNVIDIA fills the counters the proprietary driver does not expose in
// DRM sysfs. PCI identity joins nvidia-smi's order to cardN; neither index is
// stable enough to assume they are the same on a mixed-vendor machine.
func (c *Collector) collectNVIDIA(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	var targets []card
	for _, card := range c.cardSnapshot() {
		if card.nvidia && !card.asleep() {
			targets = append(targets, card)
		}
	}
	if len(targets) == 0 {
		return nil, nil
	}
	if c.smi == "" {
		return nil, errors.New("nvidia-smi was not found; install the NVIDIA user-space tools")
	}
	if since := now.Sub(c.lastSMI); !c.lastSMI.IsZero() && since >= 0 && since < nvidiaInterval {
		return nil, nil
	}
	c.lastSMI = now

	var samples []metric.Sample
	for _, target := range targets {
		if target.busID == "" {
			return samples, errors.New("NVIDIA card has no PCI bus ID")
		}

		out, err := exec.CommandContext(ctx, c.smi,
			"--id="+target.busID,
			"--query-gpu="+nvidiaQuery,
			"--format=csv,noheader,nounits",
		).Output()
		if err != nil {
			return samples, errors.Wrap(err, "reading NVIDIA metrics with nvidia-smi")
		}

		readings, err := parseNVIDIA(out, map[string]int{target.busID: target.index}, now)
		if err != nil {
			return samples, err
		}
		samples = append(samples, readings...)
	}

	return samples, nil
}

func parseNVIDIA(out []byte, byBus map[string]int, now time.Time) ([]metric.Sample, error) {
	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "parsing nvidia-smi CSV")
	}

	var samples []metric.Sample
	for _, record := range records {
		if len(record) != 7 {
			continue
		}
		index, ok := byBus[normalizeBusID(record[0])]
		if !ok {
			continue
		}

		prefix := fmt.Sprintf("gpu.%d.", index)
		for column, reading := range []struct {
			key        string
			multiplier float64
		}{
			{key: "util", multiplier: 1},
			{key: "mem.used", multiplier: 1024 * 1024},
			{key: "mem.total", multiplier: 1024 * 1024},
			{key: "temp", multiplier: 1},
			{key: "power", multiplier: 1},
			{key: "clock", multiplier: 1e6},
		} {
			value, err := strconv.ParseFloat(strings.TrimSpace(record[column+1]), 64)
			if err != nil {
				continue
			}
			samples = append(samples, metric.Sample{
				Key: series.Key(prefix + reading.key), Value: value * reading.multiplier, Time: now,
			})
		}
	}

	return samples, nil
}

func normalizeBusID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	parts := strings.Split(id, ":")
	if len(parts) == 3 {
		// NVML commonly emits an eight-digit domain while sysfs uses four.
		parts[0] = strings.TrimLeft(parts[0], "0")
		if parts[0] == "" {
			parts[0] = "0"
		}
		return strings.Join(parts, ":")
	}

	return id
}
