//go:build !linux && !windows

package page

import (
	"syscall"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/Hayao0819/hytop/internal/errors"
)

func send(choice Choice) error {
	target := choice.Identity
	if target.Started != 0 {
		item, err := process.NewProcess(int32(target.PID))
		if err != nil {
			return errors.Wrapf(err, "opening process %d", target.PID)
		}
		started, err := item.CreateTime()
		if err != nil {
			return errors.Wrapf(err, "checking process %d", target.PID)
		}
		if uint64(started) != target.Started {
			return errors.Newf("%s to %d: the PID now belongs to another process", choice.Name, target.PID)
		}
	}

	if err := syscall.Kill(target.PID, syscall.Signal(choice.Signal)); err != nil {
		switch {
		case errors.Is(err, syscall.EPERM):
			return errors.Newf("%s to %d: not yours to signal", choice.Name, target.PID)
		case errors.Is(err, syscall.ESRCH):
			return errors.Newf("%s to %d: it had already gone", choice.Name, target.PID)
		default:
			return errors.Wrapf(err, "%s to %d", choice.Name, target.PID)
		}
	}

	return nil
}
