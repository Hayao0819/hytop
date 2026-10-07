package hwmon

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "thermal.{sensor}.temp", Unit: series.Celsius},
	{Template: "thermal.max.temp", Unit: series.Celsius},
	{Template: "fan.{sensor}.rpm", Unit: series.RPM},
	{Template: "power.{sensor}.watts", Unit: series.Watts},
}

func (*Collector) Name() string { return "hwmon" }
