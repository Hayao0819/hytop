//go:build !linux

package smart

import (
	"context"
	"sync"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

type Reader struct {
	mu      sync.Mutex
	devices []diskmodel.Device
}

func NewReader(_ string, _ string) *Reader { return &Reader{} }
func (r *Reader) Devices() []diskmodel.Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]diskmodel.Device(nil), r.devices...)
}
func (*Reader) Refresh(context.Context)                          {}
func (*Reader) Read(context.Context) ([]diskmodel.Device, error) { return nil, nil }
func (r *Reader) ReplaceElevated(devices []diskmodel.Device) {
	r.mu.Lock()
	r.devices = append([]diskmodel.Device(nil), devices...)
	r.mu.Unlock()
}
