//go:build !linux

package power

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
)

type Collector struct{}

func New(string) *Collector { return &Collector{} }
func (*Collector) Check() collect.Availability {
	return collect.Availability{State: collect.NoHardware, Reason: "battery metrics are unavailable"}
}
func (*Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
func (*Collector) Facts(context.Context) (collect.Facts, error)                { return nil, nil }
