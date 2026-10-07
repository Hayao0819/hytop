//go:build linux

package smart

import (
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	smartgo "github.com/anatol/smart.go"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

func TestRawReadsTheVendorBytesTheAttributesFormatCallsFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		kind  int
		value uint64
		want  string
	}{
		{"a count is the whole 48 bits", smartgo.AtaDeviceAttributeTypeRaw48, 1234567, "1234567"},
		{"a temperature carries the range it has seen", smartgo.AtaDeviceAttributeTypeTempMinMax, 0x00002d18001f, "31 (24/45)"},
		{"a temperature that has seen no range is one number", smartgo.AtaDeviceAttributeTypeTempMinMax, 31, "31"},
		{"a spin-up time keeps its average", smartgo.AtaDeviceAttributeTypeRaw16OptAvg16, 1966<<16 | 2000, "2000 (avg 1966)"},
		{"a spin-up time without one is the current value", smartgo.AtaDeviceAttributeTypeRaw16OptAvg16, 2000, "2000"},
		{"a reallocation count drops the words left at zero", smartgo.AtaDeviceAttributeTypeRaw16OptRaw16, 5, "5"},
		{"a reallocation count keeps them once one is set", smartgo.AtaDeviceAttributeTypeRaw16OptRaw16, 3<<32 | 5, "5 3 0"},
		{"a power-on count is the low 24 bits", smartgo.AtaDeviceAttributeTypeRaw24OptRaw8, 41234, "41234"},
		{"a ratio is a pair", smartgo.AtaDeviceAttributeTypeRaw24DivRaw24, 7<<24 | 100, "7/100"},
		{"a half-minute counter reads as hours", smartgo.AtaDeviceAttributeTypeHalfMin2Hour, 240, "2 h"},
		{"a hex attribute stays hex", smartgo.AtaDeviceAttributeTypeHex48, 0xdeadbeef, "0x0000deadbeef"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := raw(smartgo.AtaSmartAttr{Type: test.kind, ValueRaw: test.value})
			if got != test.want {
				t.Fatalf("raw() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSMARTCountersSaturateInsteadOfWrapping(t *testing.T) {
	t.Parallel()

	if got := durationHours(math.MaxUint64); got < 0 || got != time.Duration(maxDurationHours)*time.Hour {
		t.Fatalf("durationHours(max) = %v", got)
	}
	maxDurationRoundedToSecond := time.Duration(math.MaxInt64/int64(time.Second)) * time.Second
	if got := hours(smartgo.AtaSmartAttr{Type: smartgo.AtaDeviceAttributeTypeSec2Hour, ValueRaw: math.MaxUint64}); got != maxDurationRoundedToSecond {
		t.Fatalf("hours(max seconds) = %v", got)
	}
	if got := uint128([2]uint64{0, 1}, 42); got != 42 {
		t.Fatalf("uint128(high word) = %d, want 42", got)
	}
	if got := dataUnits([2]uint64{math.MaxUint64, 0}); got != math.MaxUint64 {
		t.Fatalf("dataUnits(max) = %d, want %d", got, uint64(math.MaxUint64))
	}
}

func TestApplySATAOrdersAttributesAndFindsFailure(t *testing.T) {
	t.Parallel()

	device := &diskmodel.Device{}
	page := &smartgo.AtaSmartPage{Attrs: map[uint8]smartgo.AtaSmartAttr{
		194: {Id: 194, Name: "Temperature_Celsius", Current: 90, Worst: 80, ValueRaw: 35, Type: smartgo.AtaDeviceAttributeTypeTempMinMax},
		9:   {Id: 9, Name: "Power_On_Hours", Current: 10, Worst: 9, ValueRaw: 48},
	}}
	applySATA(device, page, map[uint8]uint8{9: 20})

	if device.Health != diskmodel.Failing || len(device.Attributes) != 2 ||
		device.Attributes[0].ID != 9 || !device.Attributes[0].Failing {
		t.Fatalf("SATA conversion = %+v", device)
	}
	if device.Temperature != 35 || device.PowerOnTime != 48*time.Hour {
		t.Fatalf("temperature=%v hours=%v", device.Temperature, device.PowerOnTime)
	}
}

func TestApplyNVMeConvertsHealthLog(t *testing.T) {
	t.Parallel()

	device := &diskmodel.Device{}
	log := &smartgo.NvmeSMARTLog{CritWarning: 1, Temperature: 300, PercentUsed: 7}
	log.PowerOnHours.Val[0] = 12
	log.PowerCycles.Val[0] = 3
	log.DataUnitsRead.Val[0] = 2
	log.DataUnitsWritten.Val[0] = 4
	applyNVMe(device, log)

	if device.Health != diskmodel.Failing || device.PowerOnTime != 12*time.Hour || device.PowerCycles != 3 || device.Wear != 7 {
		t.Fatalf("NVMe conversion = %+v", device)
	}
	if device.Read != 2*512*1000 || device.Written != 4*512*1000 {
		t.Fatalf("NVMe bytes = %d/%d", device.Read, device.Written)
	}
}

func TestReasonClassifiesErrorsAndStaysOnOneLine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "permission",
			err:  fmt.Errorf("open device:\n%w", os.ErrPermission),
			want: "reading SMART needs privilege: retry as administrator or grant read access to the device",
		},
		{
			name: "missing device",
			err:  fmt.Errorf("open device:\n%w", os.ErrNotExist),
			want: "no device node to read SMART from",
		},
		{
			name: "other",
			err:  errors.New("controller:\nfailed"),
			want: "controller: failed",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := reason(test.err); got != test.want {
				t.Errorf("reason(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}
