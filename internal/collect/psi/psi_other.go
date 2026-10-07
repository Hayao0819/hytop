//go:build !linux

package psi

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
)

type Collector struct{}

func New(string) (*Collector, error) { return &Collector{}, nil }
func (*Collector) Check() collect.Availability {
	return collect.Availability{State: collect.NoKernelSupport, Reason: "pressure stall information is Linux-specific"}
}
func (*Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
