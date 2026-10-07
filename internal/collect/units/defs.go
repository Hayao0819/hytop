package units

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "units.total", Unit: series.Count},
	{Template: "units.failed", Unit: series.Count},
	{Template: "units.active", Unit: series.Count},
	{Template: "units.running", Unit: series.Count, Help: "units with something actually running"},
	{Template: "units.jobs", Unit: series.Count, Help: "jobs systemd still has queued"},
	{Template: "units.{kind}", Unit: series.Count, Help: "how many units of one kind there are"},
}

func (*Collector) Name() string { return "units" }
