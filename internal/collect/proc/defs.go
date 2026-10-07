package proc

import "github.com/Hayao0819/hytop/internal/domain/series"

var Defs = []series.Def{
	{Template: "proc.count", Unit: series.Count, Help: "processes the scan found"},
	{Template: "proc.running", Unit: series.Count},
	{Template: "proc.sleeping", Unit: series.Count},
	{Template: "proc.stopped", Unit: series.Count},
	{Template: "proc.zombie", Unit: series.Count},
	{Template: "proc.threads", Unit: series.Count},
	{Template: "proc.fds", Unit: series.Count},
	{Template: "load.1", Unit: series.None, Help: "runnable and blocked tasks, averaged over a minute"},
	{Template: "load.5", Unit: series.None},
	{Template: "load.15", Unit: series.None},
	{Template: "self.cpu", Unit: series.Percent},
	{Template: "self.rss", Unit: series.Bytes},
}

func (*Collector) Name() string { return "proc" }
