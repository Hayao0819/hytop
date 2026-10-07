//go:build linux

package units

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
)

func TestSummarizeSeparatesActiveFromRunningAndCountsKinds(t *testing.T) {
	t.Parallel()

	now := time.Unix(1, 0)
	samples := summarize([]unitmodel.Unit{
		{Name: "api.service", Active: "active", Sub: "running"},
		{Name: "setup.service", Active: "active", Sub: "exited"},
		{Name: "broken.service", Active: "failed", Sub: "failed"},
		{Name: "daily.timer", Active: "active", Sub: "waiting"},
		{Name: "server.socket", Active: "active", Sub: "listening"},
		{Name: "home.mount", Active: "active", Sub: "mounted"},
	}, now)

	got := map[series.Key]float64{}
	for _, sample := range samples {
		got[sample.Key] = sample.Value
		if sample.Time != now {
			t.Errorf("%s time = %s, want %s", sample.Key, sample.Time, now)
		}
	}
	for key, want := range map[series.Key]float64{
		"units.total": 6, "units.failed": 1, "units.active": 5, "units.running": 1,
		"units.service": 3, "units.timer": 1, "units.socket": 1, "units.mount": 1,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v; samples=%v", key, got[key], want, got)
		}
	}
}
