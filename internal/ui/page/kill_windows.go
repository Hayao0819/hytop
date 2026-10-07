package page

import (
	"time"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/errors"
	"golang.org/x/sys/windows"
)

var Signals = []signalChoice{{"Terminate", 1, "terminate the process"}}

func openSignalTarget(identity procmodel.Identity) (signalTarget, error) {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE,
		false,
		uint32(identity.PID),
	)
	if err != nil {
		return signalTarget{identity: identity}, windowsSignalError("opening", identity.PID, err)
	}
	target := signalTarget{identity: identity, handle: int(handle)}

	if identity.Started != 0 {
		var creation, exit, kernel, user windows.Filetime
		if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
			closeSignalTarget(target)

			return signalTarget{identity: identity}, windowsSignalError("checking", identity.PID, err)
		}
		started := uint64(creation.Nanoseconds() / int64(time.Millisecond))
		if started != identity.Started {
			closeSignalTarget(target)

			return signalTarget{identity: identity}, errors.Newf(
				"opening process %d: the PID now belongs to another process", identity.PID,
			)
		}
	}

	return target, nil
}

func closeSignalTarget(target signalTarget) {
	if target.handle != 0 {
		_ = windows.CloseHandle(windows.Handle(target.handle))
	}
}

func send(choice Choice) error {
	target, err := openSignalTarget(choice.Identity)
	if err != nil {
		return err
	}
	defer closeSignalTarget(target)

	if err := windows.TerminateProcess(windows.Handle(target.handle), uint32(choice.Signal)); err != nil {
		return windowsSignalError(choice.Name, target.identity.PID, err)
	}

	return nil
}

func windowsSignalError(action string, pid int, err error) error {
	switch {
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		return errors.Newf("%s to %d: not yours to signal", action, pid)
	case errors.Is(err, windows.ERROR_INVALID_PARAMETER):
		return errors.Newf("%s to %d: it had already gone", action, pid)
	default:
		return errors.Wrapf(err, "%s process %d", action, pid)
	}
}
