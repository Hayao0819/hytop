package gpu

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "gpu.{n}.util", Unit: series.Percent},
	{Template: "gpu.{n}.mem.used", Unit: series.Bytes},
	{Template: "gpu.{n}.mem.total", Unit: series.Bytes},
	{Template: "gpu.{n}.temp", Unit: series.Celsius},
	{Template: "gpu.{n}.power", Unit: series.Watts},
	{Template: "gpu.{n}.clock", Unit: series.Hertz},
	{Template: "gpu.{n}.mem.clock", Unit: series.Hertz},
	{Template: "gpu.{n}.mem.gtt.used", Unit: series.Bytes},
	{Template: "gpu.{n}.mem.gtt.total", Unit: series.Bytes},
	{Template: "gpu.{n}.encode", Unit: series.Percent},
	{Template: "gpu.{n}.decode", Unit: series.Percent},
	{Template: "gpu.{n}.fan", Unit: series.Percent},
	{Template: "gpu.{n}.fan.rpm", Unit: series.RPM},
	{Template: "gpu.{n}.pcie.gen", Unit: series.None},
	{Template: "gpu.{n}.pcie.gen_max", Unit: series.None},
	{Template: "gpu.{n}.pcie.width", Unit: series.None},
	{Template: "gpu.{n}.pcie.width_max", Unit: series.None},
}

func (*Collector) Name() string { return "gpu" }
