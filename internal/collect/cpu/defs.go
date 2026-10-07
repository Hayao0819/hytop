package cpu

import "github.com/Hayao0819/hytop/internal/domain/series"

func (*Collector) StaticFacts() {}

var Defs = []series.Def{
	{Template: "cpu.total.usage", Unit: series.Percent, Help: "CPU busy across all cores"},
	{Template: "cpu.core.{n}.usage", Unit: series.Percent, Help: "one core's busy time"},
	{Template: "cpu.core.{n}.freq", Unit: series.Hertz, Help: "one core's current clock"},
	{Template: "cpu.total.freq", Unit: series.Hertz, Help: "mean clock across the cores"},
	{Template: "cpu.package.temp", Unit: series.Celsius, Help: "package temperature"},
	{Template: "cpu.package.power", Unit: series.Watts, Help: "package power from the energy counter"},
	{Template: "proc.blocked", Unit: series.Count, Help: "processes blocked on I/O"},
	{Template: "system.uptime", Unit: series.Seconds},
}

func (*Collector) Name() string { return "cpu" }
