package store_test

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/store"
)

func TestExpiredDynamicSeriesAreRemoved(t *testing.T) {
	t.Parallel()

	memory := store.New([]metric.Resolution{{Interval: time.Second, Retention: 2 * time.Second}})
	origin := time.Unix(100, 0)

	memory.WriteSamples([]metric.Sample{{Key: "net.veth-old.rx", Time: origin, Value: 1}})
	memory.WriteSamples([]metric.Sample{{Key: "cpu.total.usage", Time: origin.Add(4 * time.Second), Value: 2}})

	keys := memory.Keys()
	if len(keys) != 1 || keys[0] != "cpu.total.usage" {
		t.Fatalf("Keys() after expiry = %v", keys)
	}
	if _, ok := memory.Last("net.veth-old.rx"); ok {
		t.Fatal("expired series is still readable")
	}
}
