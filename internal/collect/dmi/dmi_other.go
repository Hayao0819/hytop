//go:build !linux

package dmi

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/metric"
)

type Collector struct{}

func New(_ string, _ string) *Collector { return &Collector{} }
func (*Collector) Name() string         { return "dmi" }
func (*Collector) Check() collect.Availability {
	return collect.Availability{State: collect.NoHardware, Reason: "SMBIOS memory details are unavailable"}
}
func (*Collector) Collect(context.Context, time.Time) ([]metric.Sample, error) { return nil, nil }
func (*Collector) Facts(context.Context) (collect.Facts, error)                { return nil, nil }
