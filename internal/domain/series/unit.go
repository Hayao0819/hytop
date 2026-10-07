package series

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Span formats a duration without redundant zero-valued suffixes.
func Span(d time.Duration) string {
	text := d.String()

	for _, whole := range []string{"h0m0s", "m0s"} {
		if strings.HasSuffix(text, whole) {
			return strings.TrimSuffix(text, whole[1:])
		}
	}

	return text
}

// Unit is what a series measures.
type Unit int

const (
	None Unit = iota
	Percent
	Bytes
	BytesPerSecond
	BitsPerSecond
	Hertz
	Watts
	Celsius
	RPM
	Seconds
	Duration
	Count
)

var unitNames = map[Unit]string{
	None:           "",
	Percent:        "%",
	Bytes:          "B",
	BytesPerSecond: "B/s",
	BitsPerSecond:  "b/s",
	Hertz:          "Hz",
	Watts:          "W",
	Celsius:        "°C",
	RPM:            "rpm",
	Seconds:        "s",
	Duration:       "",
	Count:          "",
}

func (u Unit) String() string { return unitNames[u] }

// Scale controls the representation used for a series magnitude.
type Scale int

const (
	Auto Scale = iota
	Binary
	Decimal
	BitScale
	ByteScale
	Plain
)

var scaleNames = []string{"auto", "binary", "decimal", "bits", "bytes"}

// ParseScale resolves the spelling used by configuration files.
func ParseScale(name string) (Scale, bool) {
	switch name {
	case "", "auto":
		return Auto, true
	case "binary":
		return Binary, true
	case "decimal":
		return Decimal, true
	case "bits":
		return BitScale, true
	case "bytes":
		return ByteScale, true
	default:
		return Auto, false
	}
}

// ScaleNames lists the spellings accepted in configuration files.
func ScaleNames() []string { return append([]string(nil), scaleNames...) }

// SupportsScale reports whether a representation preserves the meaning of the
// measurement. Bit/byte conversion only applies to storage and transfer rates.
func (u Unit) SupportsScale(scale Scale) bool {
	if scale == Auto {
		return true
	}

	byteLike := u == Bytes || u == BytesPerSecond || u == BitsPerSecond
	switch scale {
	case Binary, BitScale, ByteScale:
		return byteLike
	case Decimal:
		return byteLike || u == Hertz || u == Watts || u == RPM || u == Count
	default:
		return false
	}
}

var (
	binaryPrefixes  = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi"}
	decimalPrefixes = []string{"", "k", "M", "G", "T", "P"}
)

func (u Unit) Format(v float64, precision int, scale Scale) string {
	if math.IsNaN(v) {
		return "-"
	}

	if math.IsInf(v, 0) {
		return "∞"
	}

	if u == Duration {
		return formatDuration(v)
	}

	scale = u.resolve(scale)

	if scale == BitScale && (u == Bytes || u == BytesPerSecond) {
		v *= 8
	} else if scale == ByteScale && u == BitsPerSecond {
		v /= 8
	}

	value, prefix := split(v, scale)

	suffix := prefix + u.suffix(scale)
	if suffix == "" {
		return strconv.FormatFloat(value, 'f', precision, 64)
	}

	return fmt.Sprintf("%s %s", strconv.FormatFloat(value, 'f', precision, 64), suffix)
}

func (u Unit) resolve(scale Scale) Scale {
	if scale != Auto {
		return scale
	}

	switch u {
	case Bytes, BytesPerSecond:
		return Binary
	case BitsPerSecond:
		return Decimal
	case Percent, Celsius, RPM, Count, Duration:
		return Plain
	default:
		return Decimal
	}
}

func (u Unit) suffix(scale Scale) string {
	if scale == ByteScale && u == BitsPerSecond {
		return "B/s"
	}

	if scale != BitScale {
		return u.String()
	}

	switch u {
	case Bytes:
		return "b"
	case BytesPerSecond:
		return "b/s"
	default:
		return u.String()
	}
}

func split(v float64, scale Scale) (float64, string) {
	var (
		step     float64
		prefixes []string
	)

	switch scale {
	case Binary, ByteScale:
		step, prefixes = 1024, binaryPrefixes
	case Decimal, BitScale:
		step, prefixes = 1000, decimalPrefixes
	default:
		return v, ""
	}

	magnitude := math.Abs(v)
	for i, prefix := range prefixes {
		if magnitude < step || i == len(prefixes)-1 {
			return v, prefix
		}

		v, magnitude = v/step, magnitude/step
	}

	return v, ""
}

// formatDuration formats elapsed seconds as [days ]HH:MM:SS.
func formatDuration(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) {
		return "-"
	}

	total := int64(seconds)

	var (
		days  = total / 86400
		hours = (total % 86400) / 3600
		mins  = (total % 3600) / 60
		secs  = total % 60
	)

	if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, mins, secs)
	}

	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}
