package power

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "battery.{n}.capacity", Unit: series.Percent},
	{Template: "battery.{n}.power", Unit: series.Watts},
}

func (*Collector) Name() string { return "power" }
