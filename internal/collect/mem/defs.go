package mem

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "mem.total", Unit: series.Bytes},
	{Template: "mem.used", Unit: series.Bytes},
	{Template: "mem.available", Unit: series.Bytes},
	{Template: "mem.cached", Unit: series.Bytes},
	{Template: "mem.usage", Unit: series.Percent},
	{Template: "swap.total", Unit: series.Bytes},
	{Template: "swap.used", Unit: series.Bytes},
}

func (*Collector) Name() string { return "mem" }
