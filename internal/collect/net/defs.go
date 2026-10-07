package net

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "net.{iface}.rx", Unit: series.BytesPerSecond},
	{Template: "net.{iface}.tx", Unit: series.BytesPerSecond},
	{Template: "net.{iface}.rx.total", Unit: series.Bytes},
	{Template: "net.{iface}.tx.total", Unit: series.Bytes},
	{Template: "net.{iface}.speed", Unit: series.BitsPerSecond},
	{Template: "net.total.rx", Unit: series.BytesPerSecond},
	{Template: "net.total.tx", Unit: series.BytesPerSecond},
	{Template: "net.total.rx.total", Unit: series.Bytes},
	{Template: "net.total.tx.total", Unit: series.Bytes},
}

func (*Collector) Name() string { return "net" }
