package page

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/store"
	"github.com/Hayao0819/hytop/internal/ui/theme"
	"github.com/Hayao0819/hytop/internal/ui/widget/graph"
	"github.com/Hayao0819/hytop/internal/ui/widget/statgrid"
	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

var powerColours = []theme.Token{
	theme.CPU, theme.Battery, theme.DiskRead, theme.NetTx, theme.Memory, theme.Critical,
}

func powerSeries(s *store.Store) []series.Key {
	keys := s.Keys()
	patterns := []series.Pattern{"cpu.package.power", "gpu.*.power", "power.*.watts"}
	found := make([]series.Key, 0, len(keys))
	sources := map[string]bool{}
	for _, pattern := range patterns {
		for _, key := range keys {
			if !pattern.Match(key) {
				continue
			}
			source, _ := s.Fact(string(key) + ".source")
			if source != "" && sources[source] {
				continue
			}
			if source != "" {
				sources[source] = true
			}
			found = append(found, key)
		}
	}

	return found
}

func Power(env Env, keys []series.Key) reactea.Component {
	tracks := make([]graph.Track, 0, len(keys))
	stats := make([]statgrid.Stat, 0, len(keys))
	for i, key := range keys {
		tracks = append(tracks, graph.Track{
			Key: key, Colour: env.Theme.Colour(powerColours[i%len(powerColours)]),
		})
		stats = append(stats, statgrid.Stat{
			Label: powerLabel(env, key), Key: key, Unit: series.Watts, Precision: 1, Big: len(keys) <= 4,
		})
	}

	g := graph.New(env.Store, env.Caps, tracks...)
	g.Range = chart.Range{}
	g.Unit = series.Watts
	g.Label = "individual sensors; not summed"

	return Detail(env, "Power", g, statgrid.New(env.Store, stats...))
}

func powerLabel(env Env, key series.Key) string {
	segments := key.Segments()
	switch {
	case key == "cpu.package.power":
		return "CPU package"
	case len(segments) == 3 && segments[0] == "gpu":
		if name, ok := env.Store.Fact(fmt.Sprintf("gpu.name.%s", segments[1])); ok {
			return name
		}

		return "GPU " + segments[1]
	case len(segments) == 3 && segments[0] == "power":
		return strings.ReplaceAll(segments[1], "_", " ")
	default:
		return string(key)
	}
}
