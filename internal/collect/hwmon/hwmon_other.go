//go:build !linux

package hwmon

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
)

type Collector struct{}

func New(string) *Collector { return &Collector{} }
func (*Collector) Check() collect.Availability {
	return collect.Availability{State: collect.NoHardware, Reason: "hwmon is Linux-specific"}
}
func (*Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
