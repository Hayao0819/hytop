//go:build !linux

package units

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
)

type Collector struct{}

func New(string) *Collector { return &Collector{} }
func (*Collector) Check() collect.Availability {
	return collect.Availability{State: collect.NoKernelSupport, Reason: "systemd is unavailable"}
}
func (*Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
func (*Collector) Facts(context.Context) (collect.Facts, error)                { return nil, nil }
func (*Collector) Units() []unitmodel.Unit                                     { return nil }
