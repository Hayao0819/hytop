//go:build linux

package page

import (
	"syscall"

	"github.com/prometheus/procfs"
	"golang.org/x/sys/unix"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

func openSignalTarget(identity procmodel.Identity) (signalTarget, error) {
	fd, err := unix.PidfdOpen(identity.PID, 0)
	if err != nil {
		return signalTarget{identity: identity, handle: -1}, signalError("opening", identity.PID, err)
	}
	if identity.Started != 0 {
		fs, err := procfs.NewDefaultFS()
		if err != nil {
			_ = unix.Close(fd)

			return signalTarget{identity: identity, handle: -1}, errors.Wrap(err, "opening /proc")
		}
		item, err := fs.Proc(identity.PID)
		if err != nil {
			_ = unix.Close(fd)

			return signalTarget{identity: identity, handle: -1}, signalError("opening", identity.PID, err)
		}
		stat, err := item.Stat()
		if err != nil {
			_ = unix.Close(fd)

			return signalTarget{identity: identity, handle: -1}, signalError("checking", identity.PID, err)
		}
		if stat.Starttime != identity.Started {
			_ = unix.Close(fd)

			return signalTarget{identity: identity, handle: -1}, errors.Newf(
				"opening process %d: the PID now belongs to another process", identity.PID,
			)
		}
	}

	return signalTarget{identity: identity, handle: fd}, nil
}

func closeSignalTarget(target signalTarget) {
	if target.handle >= 0 {
		_ = unix.Close(target.handle)
	}
}

func send(choice Choice) error {
	target, err := openSignalTarget(choice.Identity)
	if err != nil {
		return err
	}
	defer closeSignalTarget(target)

	err = unix.PidfdSendSignal(target.handle, unix.Signal(choice.Signal), nil, 0)
	if err != nil {
		return signalError(choice.Name, target.identity.PID, err)
	}

	return nil
}

func signalError(action string, pid int, err error) error {
	switch {
	case errors.Is(err, syscall.EPERM):
		return errors.Newf("%s to %d: not yours to signal", action, pid)
	case errors.Is(err, syscall.ESRCH):
		return errors.Newf("%s to %d: it had already gone", action, pid)
	case errors.Is(err, syscall.ENOSYS):
		return errors.New("this kernel does not support safe process signaling")
	default:
		return errors.Wrapf(err, "%s process %d", action, pid)
	}
}
