//go:build linux

package page

import (
	"os"
	"testing"

	"github.com/prometheus/procfs"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func TestSignalTargetPinsAProcess(t *testing.T) {
	t.Parallel()

	fs, err := procfs.NewDefaultFS()
	if err != nil {
		t.Fatal(err)
	}
	item, err := fs.Proc(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	stat, err := item.Stat()
	if err != nil {
		t.Fatal(err)
	}

	target, err := openSignalTarget(procmodel.Identity{PID: os.Getpid(), Started: stat.Starttime})
	if err != nil {
		t.Fatal(err)
	}
	if target.handle < 0 {
		t.Fatal("pidfd was not opened")
	}
	closeSignalTarget(target)
}

func TestSignalTargetRejectsAReusedIdentity(t *testing.T) {
	t.Parallel()

	_, err := openSignalTarget(procmodel.Identity{PID: os.Getpid(), Started: ^uint64(0)})
	if err == nil {
		t.Fatal("mismatched process start time was accepted")
	}
}
