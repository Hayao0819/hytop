//go:build linux

// Package dmi reads memory-slot metadata from udev without accessing root-only
// SMBIOS tables directly.
package dmi

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Collector struct {
	database string
	syspath  string
}

func New(runRoot, sysRoot string) *Collector {
	return &Collector{
		database: filepath.Join(runRoot, "udev", "data", "+dmi:id"),
		syspath:  filepath.Join(sysRoot, "devices", "virtual", "dmi", "id"),
	}
}

func (c *Collector) Name() string { return "dmi" }

func (c *Collector) Check() collect.Availability {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	properties, err := c.properties(ctx)
	if err != nil {
		return collect.Availability{
			State:  collect.NoHardware,
			Reason: err.Error(),
			Remedy: "systemd's dmi_memory_id fills these in; a machine without udev has no source",
		}
	}

	for key := range properties {
		if strings.HasPrefix(key, "MEMORY_DEVICE_") {
			return collect.Availability{State: collect.Ready}
		}
	}

	return collect.Availability{State: collect.NoHardware, Reason: "udev knows of no memory devices"}
}

func (c *Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }

// properties prefers the database file, since reading it costs no process.
func (c *Collector) properties(ctx context.Context) (map[string]string, error) {
	if properties, err := c.fromDatabase(); err == nil {
		return properties, nil
	}

	return c.fromUdevadm(ctx)
}

// fromDatabase reads udev's own store, whose lines are tagged by kind: "E:" is
// a property.
func (c *Collector) fromDatabase() (map[string]string, error) {
	file, err := os.Open(c.database)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", c.database)
	}

	defer func() { _ = file.Close() }()

	properties := map[string]string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "E:") {
			continue
		}

		if key, value, found := strings.Cut(line[2:], "="); found {
			properties[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "reading %s", c.database)
	}

	if len(properties) == 0 {
		return nil, errors.Newf("%s held no properties", c.database)
	}

	return properties, nil
}

func (c *Collector) fromUdevadm(ctx context.Context) (map[string]string, error) {
	out, err := exec.CommandContext(ctx, "udevadm", "info", "-q", "property", "-p", c.syspath).Output()
	if err != nil {
		return nil, errors.Wrap(err, "asking udevadm for the dmi properties")
	}

	properties := map[string]string{}

	for _, line := range strings.Split(string(out), "\n") {
		if key, value, found := strings.Cut(line, "="); found {
			properties[key] = value
		}
	}

	return properties, nil
}

// module is one slot as udev describes it. An empty slot reports no size.
type module struct {
	sizeBytes  uint64
	form       string
	kind       string
	speedMTs   int
	locator    string
	partNumber string
}

func (c *Collector) Facts(ctx context.Context) (collect.Facts, error) {
	properties, err := c.properties(ctx)
	if err != nil {
		return nil, err
	}

	modules, slots := parse(properties)
	if slots == 0 {
		return nil, errors.New("udev knows of no memory devices")
	}

	facts := collect.Facts{
		series.FactMemorySlots:     strconv.Itoa(slots),
		series.FactMemorySlotsUsed: strconv.Itoa(len(modules)),
	}

	if capacity, err := strconv.ParseFloat(properties["MEMORY_ARRAY_MAX_CAPACITY"], 64); err == nil {
		facts[series.FactMemoryMaxCapacity] = series.Bytes.Format(capacity, 0, series.Auto)
	}

	if len(modules) == 0 {
		return facts, nil
	}

	facts[series.FactMemoryForm] = modules[0].form
	facts[series.FactMemoryType] = modules[0].kind
	facts[series.FactMemoryModules] = describe(modules)

	if modules[0].speedMTs > 0 {
		facts[series.FactMemorySpeed] = strconv.Itoa(modules[0].speedMTs) + " MT/s"
	}

	if modules[0].partNumber != "" {
		facts[series.FactMemoryPart] = modules[0].partNumber
	}

	return facts, nil
}

// parse groups MEMORY_DEVICE_<n>_<field> by n. It returns the populated slots
// and how many the board has in total.
func parse(properties map[string]string) ([]module, int) {
	const prefix = "MEMORY_DEVICE_"

	byIndex := map[int]map[string]string{}

	for key, value := range properties {
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		digits, field, found := strings.Cut(key[len(prefix):], "_")
		if !found {
			continue
		}

		index, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}

		if byIndex[index] == nil {
			byIndex[index] = map[string]string{}
		}

		byIndex[index][field] = value
	}

	indices := make([]int, 0, len(byIndex))
	for index := range byIndex {
		indices = append(indices, index)
	}

	sort.Ints(indices)

	modules := make([]module, 0, len(indices))

	for _, index := range indices {
		fields := byIndex[index]

		size, err := strconv.ParseUint(fields["SIZE"], 10, 64)
		if err != nil || size == 0 {
			continue
		}

		m := module{
			sizeBytes:  size,
			form:       fields["FORM_FACTOR"],
			kind:       fields["TYPE"],
			locator:    fields["LOCATOR"],
			partNumber: strings.TrimSpace(fields["PART_NUMBER"]),
		}

		// Prefer the active speed to the module's rated speed.
		for _, field := range []string{"CONFIGURED_SPEED_MTS", "SPEED_MTS"} {
			if speed, err := strconv.Atoi(fields[field]); err == nil && speed > 0 {
				m.speedMTs = speed

				break
			}
		}

		modules = append(modules, m)
	}

	return modules, len(indices)
}

// describe says "2 × 16 GiB" when the modules match and lists them when they do
// not, which is the case worth spotting.
func describe(modules []module) string {
	sizes := map[string]int{}

	for _, m := range modules {
		sizes[series.Bytes.Format(float64(m.sizeBytes), 0, series.Auto)]++
	}

	parts := make([]string, 0, len(sizes))
	for size, count := range sizes {
		parts = append(parts, strconv.Itoa(count)+" × "+size)
	}

	sort.Strings(parts)

	return strings.Join(parts, ", ")
}
