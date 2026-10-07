package power

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "battery.{n}.capacity", Unit: series.Percent},
	{Template: "battery.{n}.power", Unit: series.Watts},
	{Template: "battery.{n}.energy", Unit: series.WattHours},
	{Template: "battery.{n}.energy_full", Unit: series.WattHours},
	{Template: "battery.{n}.energy_design", Unit: series.WattHours},
	{Template: "battery.{n}.health", Unit: series.Percent},
	{Template: "battery.{n}.voltage", Unit: series.Volts},
	{Template: "battery.{n}.temp", Unit: series.Celsius},
	{Template: "battery.{n}.cycles", Unit: series.Count},
	{Template: "battery.{n}.time_to_empty", Unit: series.Duration},
	{Template: "battery.{n}.time_to_full", Unit: series.Duration},
	{Template: "battery.{n}.charge_start", Unit: series.Percent},
	{Template: "battery.{n}.charge_end", Unit: series.Percent},
}

func (*Collector) Name() string { return "power" }
