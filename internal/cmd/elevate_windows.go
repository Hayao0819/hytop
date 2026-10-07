package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

type sudoElevator struct {
	procRoot string
	sysRoot  string
}

func (sudoElevator) Available() bool { return false }

func (sudoElevator) Remove(context.Context, string, string, string) error {
	return errors.New("administrator-assisted deletion is not available on Windows")
}

func (sudoElevator) Scan(context.Context, string, string) (diskmodel.Scan, error) {
	return diskmodel.Scan{}, errors.New("administrator-assisted scanning is not available on Windows")
}

func (sudoElevator) Drives(context.Context, string) ([]diskmodel.Device, error) {
	return nil, errors.New("administrator-assisted SMART access is not available on Windows")
}

func newPrivilegedCmd() *cobra.Command {
	return &cobra.Command{Use: "_privileged", Hidden: true, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		return errors.New("the privileged helper is not available on Windows")
	}}
}

func elevator() (sudoElevator, error) { return sudoElevator{}, nil }
