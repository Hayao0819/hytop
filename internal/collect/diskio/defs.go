package diskio

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "diskio.{dev}.read", Unit: series.BytesPerSecond},
	{Template: "diskio.{dev}.write", Unit: series.BytesPerSecond},
	{Template: "diskio.{dev}.util", Unit: series.Percent},
	{Template: "diskio.total.read", Unit: series.BytesPerSecond},
	{Template: "diskio.total.write", Unit: series.BytesPerSecond},
}

func (*Collector) Name() string { return "diskio" }
