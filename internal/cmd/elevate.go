//go:build !windows

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/collect/diskusage"
	"github.com/Hayao0819/hytop/internal/collect/filesystem"
	"github.com/Hayao0819/hytop/internal/collect/smart"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

// sudoElevator runs only hytop's hidden, argument-checked helper. The password
// is never placed in argv or the environment and is discarded after the call.
type sudoElevator struct {
	executable string
	procRoot   string
	sysRoot    string
	sudoPath   string
	run        commandRunner
}

func (e sudoElevator) Available() bool { return e.sudoPath != "" }

type commandRunner func(context.Context, io.Reader, io.Writer, io.Writer, string, ...string) error

func (e sudoElevator) command(
	ctx context.Context, password string, stdout io.Writer, args ...string,
) error {
	procRoot, sysRoot := e.procRoot, e.sysRoot
	if procRoot == "" {
		procRoot = "/proc"
	}
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	argv := []string{"-S", "-p", "", "--", e.executable, "_privileged", "--proc", procRoot, "--sys", sysRoot}
	argv = append(argv, args...)
	run := e.run
	if run == nil {
		run = runCommand
	}
	var stderr bytes.Buffer
	err := run(ctx, strings.NewReader(password+"\n"), stdout, &stderr, e.sudoPath, argv...)
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}

	return nil
}

func runCommand(
	ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string,
) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

func (e sudoElevator) Remove(ctx context.Context, root, path, password string) error {
	return e.command(ctx, password, io.Discard, "remove", root, path)
}

func (e sudoElevator) Scan(ctx context.Context, root, password string) (diskmodel.Scan, error) {
	var scan diskmodel.Scan
	err := e.decode(ctx, password, &scan, "scan", root)

	return scan, err
}

func (e sudoElevator) Drives(ctx context.Context, password string) ([]diskmodel.Device, error) {
	var devices []diskmodel.Device
	err := e.decode(ctx, password, &devices, "smart")

	return devices, err
}

func (e sudoElevator) decode(ctx context.Context, password string, target any, args ...string) error {
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := e.command(ctx, password, writer, args...)
		_ = writer.CloseWithError(err)
		done <- err
		close(done)
	}()

	decodeErr := json.NewDecoder(reader).Decode(target)
	_, _ = io.Copy(io.Discard, reader)
	_ = reader.Close()
	commandErr := <-done
	if commandErr != nil {
		return commandErr
	}
	if decodeErr != nil {
		return errors.Wrap(decodeErr, "decoding privileged result")
	}

	return nil
}

func newPrivilegedCmd() *cobra.Command {
	command := &cobra.Command{
		Use:    "_privileged",
		Hidden: true,
		Args:   cobra.NoArgs,
	}

	command.AddCommand(&cobra.Command{
		Use:  "remove ROOT PATH",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return diskusage.RemoveInside(args[0], args[1], func() ([]diskmodel.Mount, error) {
				return systemMounts(cmd)
			})
		},
	})

	command.AddCommand(&cobra.Command{
		Use:  "smart",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sysRoot, _ := cmd.Flags().GetString("sys")
			reader := smart.NewReader(sysRoot, "/dev")
			devices, err := reader.Read(cmd.Context())
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(devices)
		},
	})

	command.AddCommand(&cobra.Command{
		Use:  "scan ROOT",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scan, err := diskusage.Measure(cmd.Context(), args[0], func() ([]diskmodel.Mount, error) {
				return systemMounts(cmd)
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(scan)
		},
	})

	return command
}

func systemMounts(cmd *cobra.Command) ([]diskmodel.Mount, error) {
	procRoot, _ := cmd.Flags().GetString("proc")

	return filesystem.Boundaries(procRoot)
}

func elevator() (sudoElevator, error) {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return sudoElevator{}, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return sudoElevator{}, errors.Wrap(err, "finding hytop executable")
	}

	return sudoElevator{executable: executable, procRoot: "/proc", sudoPath: sudoPath}, nil
}
