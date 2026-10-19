//go:build !linux && !windows && !darwin

package cpu

import (
	"context"

	"github.com/Hayao0819/hytop/internal/collect"
)

func platformGuestFacts(context.Context) collect.Facts { return nil }
