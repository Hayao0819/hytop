package net

import "testing"

func TestInterfaceKeyNormalizesExternalNames(t *testing.T) {
	t.Parallel()

	if got := key("Ethernet 2", "rx"); got != "net.Ethernet%202.rx" {
		t.Fatalf("key() = %q", got)
	}
}

func TestRankInterfacesPrefersCurrentTraffic(t *testing.T) {
	t.Parallel()

	got := rankInterfaces([]interfaceTraffic{
		{name: "historical", lifetime: 1_000_000},
		{name: "active", active: 50, lifetime: 100},
	})
	if len(got) != 2 || got[0] != "active" {
		t.Fatalf("rankInterfaces() = %v", got)
	}
}
