package net

import (
	"sort"

	"github.com/Hayao0819/hytop/internal/domain/series"
)

func key(iface, what string) series.Key {
	return series.Key("net." + series.NormalizeSegment(iface) + "." + what)
}

type interfaceTraffic struct {
	name     string
	active   uint64
	lifetime uint64
}

func rankInterfaces(cards []interfaceTraffic) []string {
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].active != cards[j].active {
			return cards[i].active > cards[j].active
		}
		if cards[i].lifetime != cards[j].lifetime {
			return cards[i].lifetime > cards[j].lifetime
		}

		return cards[i].name < cards[j].name
	})

	names := make([]string, 0, len(cards))
	for _, card := range cards {
		if card.lifetime > 0 {
			names = append(names, card.name)
		}
	}

	return names
}
