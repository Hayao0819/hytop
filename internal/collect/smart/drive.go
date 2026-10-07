//go:build linux

package smart

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	smartgo "github.com/anatol/smart.go"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

func (r *Reader) fromDrive(device *diskmodel.Device) {
	if device.Optical {
		device.Reason = "an optical drive reports no SMART"

		return
	}

	drive, err := smartgo.Open(filepath.Join(r.devRoot, device.Name))
	if err != nil {
		device.Reason = reason(err)

		return
	}

	defer func() { _ = drive.Close() }()

	switch drive := drive.(type) {
	case *smartgo.SataDevice:
		fromSata(device, drive)
	case *smartgo.NVMeDevice:
		fromNVMe(device, drive)
	default:
		device.Reason = drive.Type() + " drives report no SMART this can read"
	}
}

func fromSata(device *diskmodel.Device, drive *smartgo.SataDevice) {
	page, err := drive.ReadSMARTData()
	if err != nil {
		device.Reason = reason(err)

		return
	}

	// ATA health is derived from vendor attribute thresholds.
	var thresholds map[uint8]uint8

	if got, err := drive.ReadSMARTThresholds(); err == nil {
		thresholds = got.Thresholds
	}
	applySATA(device, page, thresholds)
}

func applySATA(device *diskmodel.Device, page *smartgo.AtaSmartPage, thresholds map[uint8]uint8) {
	device.Health = diskmodel.Passed

	for _, id := range ids(page.Attrs) {
		attribute := page.Attrs[id]
		threshold := int(thresholds[id])
		failing := threshold > 0 && int(attribute.Current) <= threshold

		if failing {
			device.Health = diskmodel.Failing
		}

		device.Attributes = append(device.Attributes, diskmodel.Attribute{
			ID:        int(attribute.Id),
			Name:      strings.ReplaceAll(attribute.Name, "_", " "),
			Value:     int(attribute.Current),
			Worst:     int(attribute.Worst),
			Threshold: threshold,
			Raw:       raw(attribute),
			Failing:   failing,
		})

		switch attribute.Name {
		case "Temperature_Celsius", "Temperature_Celsius_X10":
			if got := celsius(attribute); got != 0 {
				device.Temperature = got
			}

		case "Airflow_Temperature_Cel":
			if device.Temperature == 0 {
				device.Temperature = celsius(attribute)
			}

		case "Power_On_Hours":
			device.PowerOnTime = hours(attribute)
		case "Power_Cycle_Count":
			device.PowerCycles = attribute.ValueRaw
		}
	}
}

func fromNVMe(device *diskmodel.Device, drive *smartgo.NVMeDevice) {
	log, err := drive.ReadSMART()
	if err != nil {
		device.Reason = reason(err)

		return
	}
	applyNVMe(device, log)
}

func applyNVMe(device *diskmodel.Device, log *smartgo.NvmeSMARTLog) {
	device.Health = diskmodel.Passed
	if log.CritWarning != 0 {
		device.Health = diskmodel.Failing
	}

	// NVMe temperature is Kelvin; zero means unavailable.
	if log.Temperature > 0 {
		device.Temperature = float64(log.Temperature) - 273.15
	}

	device.PowerOnTime = durationHours(uint128(log.PowerOnHours.Val, maxDurationHours))
	device.PowerCycles = uint128(log.PowerCycles.Val, math.MaxUint64)
	device.Wear = float64(log.PercentUsed)

	device.Read = dataUnits(log.DataUnitsRead.Val)
	device.Written = dataUnits(log.DataUnitsWritten.Val)
}

func ids(attributes map[uint8]smartgo.AtaSmartAttr) []uint8 {
	sorted := slices.Collect(maps.Keys(attributes))
	slices.Sort(sorted)

	return sorted
}

func celsius(attribute smartgo.AtaSmartAttr) float64 {
	got, _, _, _, err := attribute.ParseAsTemperature()
	if err != nil {
		return 0
	}

	return float64(got)
}

func hours(attribute smartgo.AtaSmartAttr) time.Duration {
	switch attribute.Type {
	case smartgo.AtaDeviceAttributeTypeMin2Hour:
		return duration(attribute.ValueRaw&0xffffffff, time.Minute)
	case smartgo.AtaDeviceAttributeTypeSec2Hour:
		return duration(attribute.ValueRaw, time.Second)
	case smartgo.AtaDeviceAttributeTypeHalfMin2Hour:
		return duration(attribute.ValueRaw, 30*time.Second)
	case smartgo.AtaDeviceAttributeTypeMsec24Hour32:
		return addDuration(
			duration(attribute.ValueRaw&0xffffffff, time.Hour),
			duration(attribute.ValueRaw>>32, time.Millisecond),
		)
	}

	// Unspecified SMART counters conventionally store hours in the low 24 bits.
	return durationHours(bits(attribute.ValueRaw, 0, 24))
}

const maxDurationHours = uint64(math.MaxInt64 / int64(time.Hour))

func durationHours(value uint64) time.Duration {
	return duration(value, time.Hour)
}

func duration(value uint64, unit time.Duration) time.Duration {
	limit := uint64(math.MaxInt64 / int64(unit))

	return time.Duration(min(value, limit)) * unit
}

func addDuration(left, right time.Duration) time.Duration {
	if left > time.Duration(math.MaxInt64)-right {
		return time.Duration(math.MaxInt64)
	}

	return left + right
}

func uint128(value [2]uint64, limit uint64) uint64 {
	if value[1] != 0 || value[0] > limit {
		return limit
	}

	return value[0]
}

func dataUnits(value [2]uint64) uint64 {
	// NVMe data units are 1000 sectors of 512 bytes.
	const unit = uint64(512 * 1000)

	if value[1] != 0 || value[0] > math.MaxUint64/unit {
		return math.MaxUint64
	}

	return value[0] * unit
}

// raw formats vendor bytes using smart.go's smartmontools-derived metadata.
func raw(attribute smartgo.AtaSmartAttr) string {
	value := attribute.ValueRaw

	switch attribute.Type {
	case smartgo.AtaDeviceAttributeTypeTempMinMax, smartgo.AtaDeviceAttributeTypeTemp10X:
		celsius, low, high, _, err := attribute.ParseAsTemperature()
		if err != nil {
			break
		}

		if low == 0 && high == 0 {
			return strconv.Itoa(celsius)
		}

		return fmt.Sprintf("%d (%d/%d)", celsius, low, high)

	case smartgo.AtaDeviceAttributeTypeSec2Hour, smartgo.AtaDeviceAttributeTypeMin2Hour,
		smartgo.AtaDeviceAttributeTypeHalfMin2Hour, smartgo.AtaDeviceAttributeTypeMsec24Hour32:
		return fmt.Sprintf("%.0f h", hours(attribute).Hours())

	case smartgo.AtaDeviceAttributeTypeRaw8:
		return fmt.Sprintf("%d %d %d %d %d %d", bits(value, 40, 8), bits(value, 32, 8),
			bits(value, 24, 8), bits(value, 16, 8), bits(value, 8, 8), bits(value, 0, 8))

	case smartgo.AtaDeviceAttributeTypeRaw16:
		return fmt.Sprintf("%d %d %d", bits(value, 32, 16), bits(value, 16, 16), bits(value, 0, 16))

	case smartgo.AtaDeviceAttributeTypeRaw16OptRaw16:
		return optional(bits(value, 0, 16), bits(value, 32, 16), bits(value, 16, 16))

	case smartgo.AtaDeviceAttributeTypeRaw16OptAvg16:
		if average := bits(value, 16, 16); average != 0 {
			return fmt.Sprintf("%d (avg %d)", bits(value, 0, 16), average)
		}

		return strconv.FormatUint(bits(value, 0, 16), 10)

	case smartgo.AtaDeviceAttributeTypeRaw24OptRaw8:
		return optional(bits(value, 0, 24), bits(value, 40, 8), bits(value, 32, 8), bits(value, 24, 8))

	case smartgo.AtaDeviceAttributeTypeRaw24DivRaw24:
		return fmt.Sprintf("%d/%d", bits(value, 24, 24), bits(value, 0, 24))

	case smartgo.AtaDeviceAttributeTypeRaw24DivRaw32:
		return fmt.Sprintf("%d/%d", bits(value, 32, 24), bits(value, 0, 32))

	case smartgo.AtaDeviceAttributeTypeHex48:
		return fmt.Sprintf("0x%012x", value)

	case smartgo.AtaDeviceAttributeTypeHex56:
		return fmt.Sprintf("0x%014x", value)

	case smartgo.AtaDeviceAttributeTypeHex64:
		return fmt.Sprintf("0x%016x", value)
	}

	return strconv.FormatUint(value, 10)
}

func optional(value uint64, rest ...uint64) string {
	words := []string{strconv.FormatUint(value, 10)}

	if slices.ContainsFunc(rest, func(word uint64) bool { return word != 0 }) {
		for _, word := range rest {
			words = append(words, strconv.FormatUint(word, 10))
		}
	}

	return strings.Join(words, " ")
}

func bits(value uint64, offset, width int) uint64 {
	return (value >> offset) & (1<<width - 1)
}

func reason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "reading SMART needs privilege: retry as administrator or grant read access to the device"
	case errors.Is(err, os.ErrNotExist):
		return "no device node to read SMART from"
	}

	return strings.Join(strings.Fields(err.Error()), " ")
}
