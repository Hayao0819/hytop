//go:build !windows

package cmd

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSudoElevatorKeepsPasswordOutOfArguments(t *testing.T) {
	t.Parallel()

	var gotName, gotStdin string
	var gotArgs []string
	e := sudoElevator{
		executable: "/usr/bin/hytop",
		procRoot:   "/host/proc",
		sudoPath:   "/usr/bin/sudo",
		run: func(_ context.Context, stdin io.Reader, stdout, _ io.Writer, name string, args ...string) error {
			input, _ := io.ReadAll(stdin)
			gotName, gotStdin, gotArgs = name, string(input), slices.Clone(args)
			_, _ = io.WriteString(stdout, `{"root":"/tmp"}`)

			return nil
		},
	}

	if _, err := e.Scan(context.Background(), "/tmp", "secret"); err != nil {
		t.Fatal(err)
	}
	if gotName != "/usr/bin/sudo" || gotStdin != "secret\n" {
		t.Fatalf("command name=%q stdin=%q", gotName, gotStdin)
	}
	want := []string{"-S", "-p", "", "--", "/usr/bin/hytop", "_privileged", "--proc", "/host/proc", "--sys", "/sys", "scan", "/tmp"}
	if !slices.Equal(gotArgs, want) {
		t.Fatalf("arguments %q, want %q", gotArgs, want)
	}
	if strings.Contains(strings.Join(gotArgs, " "), "secret") {
		t.Fatal("password leaked into argv")
	}
}

func TestSudoElevatorReportsStderrAndBadJSON(t *testing.T) {
	t.Parallel()

	failing := sudoElevator{sudoPath: "/usr/bin/sudo", run: func(_ context.Context, _ io.Reader, _, stderr io.Writer, _ string, _ ...string) error {
		_, _ = io.WriteString(stderr, "sudo: denied\n")

		return stderrors.New("exit status 1")
	}}
	if err := failing.Remove(context.Background(), "/", "/tmp/x", "bad"); err == nil || err.Error() != "sudo: denied" {
		t.Fatalf("Remove error = %v", err)
	}

	broken := sudoElevator{sudoPath: "/usr/bin/sudo", run: func(_ context.Context, _ io.Reader, stdout, _ io.Writer, _ string, _ ...string) error {
		_, _ = io.WriteString(stdout, "not json")

		return nil
	}}
	if _, err := broken.Scan(context.Background(), "/", "secret"); err == nil || !strings.Contains(err.Error(), "decoding privileged result") {
		t.Fatalf("Scan error = %v", err)
	}
}

func TestSudoElevatorDecodesDriveInformation(t *testing.T) {
	t.Parallel()

	e := sudoElevator{sudoPath: "/usr/bin/sudo", run: func(_ context.Context, _ io.Reader, stdout, _ io.Writer, _ string, _ ...string) error {
		_, _ = io.WriteString(stdout, `[{"Name":"nvme0n1","Model":"Fast"}]`)

		return nil
	}}

	drives, err := e.Drives(context.Background(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(drives) != 1 || drives[0].Name != "nvme0n1" || drives[0].Model != "Fast" {
		t.Fatalf("drives = %+v", drives)
	}
}

func TestSudoElevatorIsAvailableOnlyWithSudo(t *testing.T) {
	t.Parallel()

	if (sudoElevator{}).Available() {
		t.Fatal("an elevator without sudo is available")
	}
	if !(sudoElevator{sudoPath: "/usr/bin/sudo"}).Available() {
		t.Fatal("an elevator with sudo is unavailable")
	}
}

func TestElevatorFindsSudoOnPath(t *testing.T) {
	dir := t.TempDir()
	sudoPath := filepath.Join(dir, "sudo")
	if err := os.WriteFile(sudoPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	e, err := elevator()
	if err != nil {
		t.Fatal(err)
	}
	if !e.Available() || e.sudoPath != sudoPath {
		t.Fatalf("elevator = %+v, want sudo at %q", e, sudoPath)
	}

	t.Setenv("PATH", t.TempDir())
	e, err = elevator()
	if err != nil {
		t.Fatal(err)
	}
	if e.Available() {
		t.Fatal("elevator is available without sudo")
	}
}
