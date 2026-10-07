package psi

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "psi.{resource}.some.avg10", Unit: series.Percent},
	{Template: "psi.{resource}.some.avg60", Unit: series.Percent},
	{Template: "psi.{resource}.some.avg300", Unit: series.Percent},
	{Template: "psi.{resource}.full.avg10", Unit: series.Percent},
	{Template: "psi.{resource}.full.avg60", Unit: series.Percent},
	{Template: "psi.{resource}.full.avg300", Unit: series.Percent},
}

func (*Collector) Name() string { return "psi" }
