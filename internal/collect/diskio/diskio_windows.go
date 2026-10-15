package diskio

import (
	"github.com/yusufpapurcu/wmi"

	"github.com/Hayao0819/hytop/internal/collect"
)

type Collector struct {
	*performanceCollector
}

func New(_, _ string) (*Collector, error) {
	return &Collector{newPerformanceCollector(readPhysicalDisks)}, nil
}

func (*Collector) Check() collect.Availability { return collect.Availability{State: collect.Ready} }

func readPhysicalDisks() ([]performanceCounters, error) {
	var counters []performanceCounters
	err := wmi.QueryNamespace(
		"SELECT Name, DiskReadBytesPerSec, DiskWriteBytesPerSec, PercentIdleTime, PercentIdleTime_Base, Timestamp_Sys100NS "+
			"FROM Win32_PerfRawData_PerfDisk_PhysicalDisk WHERE Name <> '_Total'",
		&counters, `ROOT\CIMV2`,
	)

	return counters, err
}
