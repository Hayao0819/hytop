package series_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestParseKeyRejectsWhatWouldBeAmbiguous(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"", "cpu..usage", "cpu.*.usage", "cpu.**", "cpu.u*age", "cpu.{n}.usage", "cpu.n}.usage"} {
		if _, err := series.ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) = nil error, want one", bad)
		}
	}

	if _, err := series.ParseKey("cpu.core.3.usage"); err != nil {
		t.Errorf("ParseKey: %v", err)
	}
}

func TestParsePatternAndRegistryUnit(t *testing.T) {
	t.Parallel()

	for _, good := range []string{"cpu.total.usage", "cpu.core.*.usage", "cpu.**", "**"} {
		if _, err := series.ParsePattern(good); err != nil {
			t.Errorf("ParsePattern(%q): %v", good, err)
		}
	}
	for _, bad := range []string{"", "cpu..usage", "cpu.**.usage", "cpu.u*age", "cpu.{n}.usage", "cpu.n}.usage"} {
		if _, err := series.ParsePattern(bad); err == nil {
			t.Errorf("ParsePattern(%q) = nil error, want one", bad)
		}
	}

	registry := &series.Registry{}
	registry.MustRegister(
		series.Def{Template: "cpu.total.usage", Unit: series.Percent},
		series.Def{Template: "cpu.core.{n}.usage", Unit: series.Percent},
		series.Def{Template: "cpu.model.code", Unit: series.Count},
	)

	pattern, _ := series.ParsePattern("cpu.core.*.usage")
	if unit, ok := registry.PatternUnit(pattern); !ok || unit != series.Percent {
		t.Errorf("PatternUnit(%q) = %v, %v", pattern, unit, ok)
	}
	pattern, _ = series.ParsePattern("cpu.**")
	if _, ok := registry.PatternUnit(pattern); ok {
		t.Error("a selector spanning percent and count should have no common unit")
	}
	pattern, _ = series.ParsePattern("mem.**")
	if _, ok := registry.PatternUnit(pattern); ok {
		t.Error("an unknown selector should have no unit")
	}
}

func TestNormalizeSegmentIsReversibleInPractice(t *testing.T) {
	t.Parallel()

	inputs := []string{"eth.0", "eth_0", "/", "/root", "a/b", "a-b", "日本語"}
	seen := make(map[string]string, len(inputs))
	for _, input := range inputs {
		normalized := series.NormalizeSegment(input)
		if strings.Contains(normalized, ".") {
			t.Errorf("NormalizeSegment(%q) left a separator in %q", input, normalized)
		}
		if previous, ok := seen[normalized]; ok {
			t.Errorf("NormalizeSegment collides for %q and %q as %q", previous, input, normalized)
		}
		seen[normalized] = input
	}
}

func TestPatternMatchesOneSegmentPerStar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		key     string
		want    bool
	}{
		{"cpu.core.*.usage", "cpu.core.3.usage", true},
		{"cpu.core.*.usage", "cpu.core.3.4.usage", false},
		{"cpu.core.*.usage", "cpu.total.usage", false},
		{"cpu.total.usage", "cpu.total.usage", true},
		{"diskio.**", "diskio.nvme0n1.read", true},
		{"diskio.**", "diskio", false},
		{"*.total.usage", "cpu.total.usage", true},
	}

	for _, tt := range tests {
		if got := series.Pattern(tt.pattern).Match(series.Key(tt.key)); got != tt.want {
			t.Errorf("Pattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.key, got, tt.want)
		}
	}
}

func TestPatternExpandKeepsTheGivenOrder(t *testing.T) {
	t.Parallel()

	live := []series.Key{"cpu.core.1.usage", "cpu.total.usage", "cpu.core.0.usage"}

	got := series.Pattern("cpu.core.*.usage").Expand(live)

	want := []series.Key{"cpu.core.1.usage", "cpu.core.0.usage"}
	if len(got) != len(want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Expand = %v, want %v", got, want)
		}
	}
}

func TestRegistryCapturesPlaceholders(t *testing.T) {
	t.Parallel()

	var registry series.Registry

	registry.MustRegister(
		series.Def{Template: "cpu.total.usage", Unit: series.Percent},
		series.Def{Template: "cpu.core.{n}.usage", Unit: series.Percent},
		series.Def{Template: "diskio.{dev}.read", Unit: series.BytesPerSecond},
	)

	def, captured, ok := registry.Lookup("cpu.core.7.usage")
	if !ok {
		t.Fatal("Lookup(cpu.core.7.usage) not found")
	}

	if def.Unit != series.Percent {
		t.Errorf("unit = %v, want Percent", def.Unit)
	}

	if captured["n"] != "7" {
		t.Errorf("captured = %v, want n=7", captured)
	}

	if _, _, ok := registry.Lookup("cpu.core.7.freq"); ok {
		t.Error("Lookup(cpu.core.7.freq) found, want not found")
	}

	if got := registry.Unit("nothing.here"); got != series.None {
		t.Errorf("Unit(unregistered) = %v, want None", got)
	}
}

func TestRegistryRejectsDuplicatesAndBadTemplates(t *testing.T) {
	t.Parallel()

	var registry series.Registry

	registry.MustRegister(series.Def{Template: "cpu.total.usage"})

	if err := registry.Register(series.Def{Template: "cpu.total.usage"}); err == nil {
		t.Error("registering the same template twice = nil error, want one")
	}

	for _, bad := range []string{"", "cpu..usage", "cpu.*.usage", "cpu.u*age", "cpu.{n.usage"} {
		if err := registry.Register(series.Def{Template: bad}); err == nil {
			t.Errorf("Register(%q) = nil error, want one", bad)
		}
	}
}

func TestDefPatternMatchesItsOwnInstances(t *testing.T) {
	t.Parallel()

	def := series.Def{Template: "gpu.{n}.mem.used"}

	if got := def.Pattern(); got != "gpu.*.mem.used" {
		t.Fatalf("Pattern = %q", got)
	}

	if !def.Pattern().Match("gpu.0.mem.used") {
		t.Error("a def's pattern does not match its own instance")
	}
}

func TestUnitFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		unit      series.Unit
		value     float64
		precision int
		scale     series.Scale
		want      string
	}{
		{series.Percent, 42.5, 1, series.Auto, "42.5 %"},
		{series.Bytes, 1536, 1, series.Auto, "1.5 KiB"},
		{series.Bytes, 1536, 1, series.Decimal, "1.5 kB"},
		{series.BytesPerSecond, 125_000, 0, series.BitScale, "1 Mb/s"},
		{series.BitsPerSecond, 1_000_000, 0, series.BitScale, "1 Mb/s"},
		{series.BitsPerSecond, 8_388_608, 0, series.ByteScale, "1 MiB/s"},
		{series.Hertz, 3_600_000_000, 2, series.Auto, "3.60 GHz"},
		{series.Celsius, 71.4, 0, series.Auto, "71 °C"},
		{series.Count, 12, 0, series.Auto, "12"},
		{series.None, 0.5, 2, series.Auto, "0.50"},
		{series.Bytes, 0, 0, series.Auto, "0 B"},
	}

	for _, tt := range tests {
		if got := tt.unit.Format(tt.value, tt.precision, tt.scale); got != tt.want {
			t.Errorf("%v.Format(%v, %d, %v) = %q, want %q",
				tt.unit, tt.value, tt.precision, tt.scale, got, tt.want)
		}
	}
}

func TestConfiguredScalesPreserveUnitMeaning(t *testing.T) {
	t.Parallel()

	for _, name := range series.ScaleNames() {
		if _, ok := series.ParseScale(name); !ok {
			t.Errorf("ParseScale(%q) = false", name)
		}
	}
	if _, ok := series.ParseScale("blocks"); ok {
		t.Error("ParseScale(blocks) = true")
	}

	if series.Percent.SupportsScale(series.BitScale) {
		t.Error("percent accepted a bit scale")
	}
	if !series.BytesPerSecond.SupportsScale(series.BitScale) {
		t.Error("a byte rate refused conversion to bits")
	}
	if !series.BitsPerSecond.SupportsScale(series.ByteScale) {
		t.Error("a bit rate refused conversion to bytes")
	}
}
