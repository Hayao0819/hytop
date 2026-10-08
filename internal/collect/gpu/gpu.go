//go:build linux

// Package gpu reads DRM device information from sysfs and vendor tools.
package gpu

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/collect/sysread"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type card struct {
	index  int
	dir    string
	name   string
	busID  string
	nvidia bool
}

type Collector struct {
	sysRoot       string
	runRoot       string
	mu            sync.RWMutex
	cards         []card
	lastDiscovery time.Time
	discoveryErr  error
	smi           string
	lastSMI       time.Time
}

const cardDiscoveryInterval = 10 * time.Second

func New(sysRoot string, runRoot ...string) *Collector {
	run := "/run"
	if len(runRoot) > 0 {
		run = runRoot[0]
	}

	c := &Collector{sysRoot: sysRoot, runRoot: run}
	_, _ = c.refreshCards(time.Now())
	c.smi, _ = exec.LookPath("nvidia-smi")

	return c
}

func (c *Collector) Check() collect.Availability {
	cards, err := c.refreshCards(time.Now())
	if err != nil {
		return collect.Availability{State: collect.NoHardware, Reason: err.Error()}
	}
	if len(cards) == 0 {
		return collect.Availability{
			State:  collect.NoHardware,
			Reason: "no DRM cards were found in sysfs",
			Remedy: "check that a DRM driver is loaded and /sys/class/drm is accessible",
		}
	}

	return collect.Availability{State: collect.Ready}
}

func (c *Collector) refreshCards(now time.Time) ([]card, error) {
	cards, err := c.find()

	c.mu.Lock()
	if err == nil {
		c.cards = cards
	}
	c.lastDiscovery = now
	c.discoveryErr = err
	snapshot := slices.Clone(c.cards)
	c.mu.Unlock()

	return snapshot, err
}

func (c *Collector) currentCards(now time.Time) ([]card, error) {
	c.mu.RLock()
	if now.Before(c.lastDiscovery.Add(cardDiscoveryInterval)) {
		cards := slices.Clone(c.cards)
		err := c.discoveryErr
		c.mu.RUnlock()

		return cards, err
	}
	c.mu.RUnlock()

	return c.refreshCards(now)
}

func (c *Collector) cardSnapshot() []card {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.cards)
}

// find walks /sys/class/drm for cardN that has a device behind it. Rendering
// nodes and connectors are skipped: only the card itself has the counters.
func (c *Collector) find() ([]card, error) {
	root := filepath.Join(c.sysRoot, "class", "drm")

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var found []card

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}

		index, err := strconv.Atoi(strings.TrimPrefix(name, "card"))
		if err != nil {
			continue
		}

		device := filepath.Join(root, name, "device")
		if _, err := os.Stat(device); err != nil {
			continue
		}

		target, _ := filepath.EvalSymlinks(device)
		vendor, _ := sysread.String(filepath.Join(device, "vendor"))
		found = append(found, card{
			index: index, dir: device, name: c.cardName(device),
			busID: normalizeBusID(filepath.Base(target)), nvidia: vendor == "0x10de",
		})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].index < found[j].index })

	return found, nil
}

// asleep reports a card in a low-power state so sysfs reads do not wake it.
func (c card) asleep() bool {
	state, ok := sysread.String(filepath.Join(c.dir, "power_state"))

	return ok && state != "D0"
}

func (c *Collector) Collect(ctx context.Context, now time.Time) ([]metric.Sample, error) {
	cards, err := c.currentCards(now)
	if err != nil {
		return nil, errors.Wrap(err, "discovering DRM cards")
	}
	if len(cards) == 0 {
		return nil, nil
	}

	var (
		samples []metric.Sample
		awake   bool
	)

	for _, card := range cards {
		id := strconv.Itoa(card.index)

		if card.asleep() {
			continue
		}
		awake = true

		samples = readSamples(samples, card.dir, "gpu."+id+".", now, []sensorReading{
			{"util", []string{"gpu_busy_percent"}, 1},
			{"mem.used", []string{"mem_info_vram_used"}, 1},
			{"mem.total", []string{"mem_info_vram_total"}, 1},
			{"mem.gtt.used", []string{"mem_info_gtt_used"}, 1},
			{"mem.gtt.total", []string{"mem_info_gtt_total"}, 1},
		})

		samples = append(samples, card.hwmon(id, now)...)
	}

	nvidia, nvidiaErr := c.collectNVIDIA(ctx, now)
	samples = append(samples, nvidia...)
	if nvidiaErr != nil {
		return samples, nvidiaErr
	}

	if len(samples) == 0 {
		if len(cards) > 0 && !awake {
			return nil, nil
		}

		return nil, errors.New("no card reported anything")
	}

	return samples, nil
}

// hwmon is where a DRM driver puts its temperature, power draw and clocks.
func (c card) hwmon(id string, now time.Time) []metric.Sample {
	dir := c.hwmonDir()
	if dir == "" {
		return nil
	}

	samples := readSamples(nil, dir, "gpu."+id+".", now, []sensorReading{
		{"temp", []string{"temp1_input"}, 1000},
		{"power", []string{"power1_average", "power1_input"}, 1e6},
		{"clock", []string{"freq1_input"}, 1},
		{"fan.rpm", []string{"fan1_input"}, 1},
	})

	for n := 1; n <= 8; n++ {
		if label, ok := sysread.String(filepath.Join(dir, "freq"+strconv.Itoa(n)+"_label")); ok && label == "mclk" {
			if value, ok := sysread.Float(filepath.Join(dir, "freq"+strconv.Itoa(n)+"_input")); ok {
				samples = append(samples, metric.Sample{
					Key: series.Key("gpu." + id + ".mem.clock"), Value: value, Time: now,
				})
			}
			break
		}
	}

	return samples
}

func (c card) hwmonDir() string {
	root := filepath.Join(c.dir, "hwmon")

	entries, err := os.ReadDir(root)
	if err != nil || len(entries) == 0 {
		return ""
	}

	return filepath.Join(root, entries[0].Name())
}

type sensorReading struct {
	key     string
	files   []string
	divisor float64
}

func readSamples(samples []metric.Sample, dir, prefix string, now time.Time, readings []sensorReading) []metric.Sample {
	for _, reading := range readings {
		for _, file := range reading.files {
			if value, ok := sysread.Float(filepath.Join(dir, file)); ok {
				samples = append(samples, metric.Sample{
					Key: series.Key(prefix + reading.key), Value: value / reading.divisor, Time: now,
				})
				break
			}
		}
	}

	return samples
}

func (c *Collector) Facts(context.Context) (collect.Facts, error) {
	cards := c.cardSnapshot()
	if len(cards) == 0 {
		return collect.Facts{}, nil
	}

	facts := collect.Facts{}

	for _, card := range cards {
		id := strconv.Itoa(card.index)
		facts[series.FactGPUName+"."+id] = card.name
		if !card.asleep() {
			link := pcieLink(card.dir, "current")
			if link != "" {
				facts["gpu."+id+".pcie.current"] = link
			}
			maximum := pcieLink(card.dir, "max")
			if maximum != "" {
				facts["gpu."+id+".pcie.max"] = maximum
			}
		}
	}

	facts[series.FactGPUName] = cards[0].name

	return facts, nil
}

func pcieLink(dir, kind string) string {
	speed, _ := sysread.String(filepath.Join(dir, kind+"_link_speed"))
	width, _ := sysread.String(filepath.Join(dir, kind+"_link_width"))
	if speed == "" {
		return ""
	}
	if width != "" {
		return speed + " ×" + width
	}

	return speed
}

// Cards returns discovered DRM card indices.
func (c *Collector) Cards() []int {
	cards := c.cardSnapshot()
	indices := make([]int, 0, len(cards))
	for _, card := range cards {
		indices = append(indices, card.index)
	}

	return indices
}

// cardName uses udev's PCI database result when available. This is the same
// local, unprivileged source desktop device managers use and avoids carrying a
// second copy of pci.ids. Raw IDs remain a reliable fallback on minimal hosts.
func (c *Collector) cardName(dir string) string {
	if target, err := filepath.EvalSymlinks(dir); err == nil {
		properties := udevProperties(filepath.Join(c.runRoot, "udev", "data", "+pci:"+filepath.Base(target)))
		if model := properties["ID_MODEL_FROM_DATABASE"]; model != "" {
			return model
		}
	}

	vendors := map[string]string{
		"0x1002": "AMD", "0x10de": "NVIDIA", "0x8086": "Intel",
	}

	vendor, _ := sysread.String(filepath.Join(dir, "vendor"))
	device, _ := sysread.String(filepath.Join(dir, "device"))

	name, ok := vendors[vendor]
	if !ok {
		name = vendor
	}

	if device != "" {
		return name + " " + device
	}

	return name
}

func udevProperties(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	properties := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "E:") {
			continue
		}
		if key, value, ok := strings.Cut(line[2:], "="); ok {
			properties[key] = value
		}
	}

	return properties
}
