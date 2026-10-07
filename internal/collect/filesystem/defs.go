package filesystem

import (
	"time"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

var Defs = []series.Def{
	{Template: "fs.{mount}.used", Unit: series.Bytes, Help: "space in use on one filesystem"},
	{Template: "fs.{mount}.free", Unit: series.Bytes},
	{Template: "fs.{mount}.usage", Unit: series.Percent},
	{Template: "fs.total.used", Unit: series.Bytes},
	{Template: "fs.total.size", Unit: series.Bytes},
}

func Escape(path string) string {
	return series.NormalizeSegment(path)
}

func (*Collector) Name() string { return "filesystem" }

func buildSamples(mounts []diskmodel.Mount, now time.Time, count func(diskmodel.Mount) bool) []metric.Sample {
	samples := make([]metric.Sample, 0, len(mounts)*3+2)
	seen := make(map[string]bool, len(mounts))
	var total, used uint64
	for _, mount := range mounts {
		key := Escape(mount.Path)
		samples = append(samples,
			metric.Sample{Key: series.Key("fs." + key + ".used"), Value: float64(mount.Used()), Time: now},
			metric.Sample{Key: series.Key("fs." + key + ".free"), Value: float64(mount.Available), Time: now},
			metric.Sample{Key: series.Key("fs." + key + ".usage"), Value: mount.Usage(), Time: now},
		)
		if seen[mount.Device] || !count(mount) {
			continue
		}
		seen[mount.Device] = true
		total += mount.Total
		used += mount.Used()
	}

	return append(samples,
		metric.Sample{Key: "fs.total.used", Value: float64(used), Time: now},
		metric.Sample{Key: "fs.total.size", Value: float64(total), Time: now},
	)
}
