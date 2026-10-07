package gpu

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "gpu.{n}.util", Unit: series.Percent},
	{Template: "gpu.{n}.mem.used", Unit: series.Bytes},
	{Template: "gpu.{n}.mem.total", Unit: series.Bytes},
	{Template: "gpu.{n}.temp", Unit: series.Celsius},
	{Template: "gpu.{n}.power", Unit: series.Watts},
	{Template: "gpu.{n}.clock", Unit: series.Hertz},
}

func (*Collector) Name() string { return "gpu" }
