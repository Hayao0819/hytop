//go:build linux

package psi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestCollectorReadsSomeAndFullPressure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pressure"), 0o755); err != nil {
		t.Fatal(err)
	}
	for resource, contents := range map[string]string{
		"cpu":    "some avg10=1.25 avg60=2.50 avg300=3.75 total=100\n",
		"io":     "some avg10=4.00 avg60=5.00 avg300=6.00 total=200\nfull avg10=0.50 avg60=1.00 avg300=1.50 total=50\n",
		"memory": "some avg10=7.00 avg60=8.00 avg300=9.00 total=300\nfull avg10=2.00 avg60=2.50 avg300=3.00 total=80\n",
	} {
		if err := os.WriteFile(filepath.Join(root, "pressure", resource), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	collector, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if availability := collector.Check(); availability.State != collect.Ready {
		t.Fatalf("availability = %+v", availability)
	}
	samples, err := collector.Collect(t.Context(), time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}

	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
	}
	for key, want := range map[series.Key]float64{
		"psi.cpu.some.avg10":     1.25,
		"psi.io.full.avg60":      1,
		"psi.memory.some.avg300": 9,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; samples=%v", key, got[key], want, got)
		}
	}
	if _, exists := got["psi.cpu.full.avg10"]; exists {
		t.Fatalf("collector invented CPU full pressure: %v", got)
	}
}
