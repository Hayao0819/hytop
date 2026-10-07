//go:build linux

package diskusage

import (
	"context"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

type heldElevator struct {
	started chan struct{}
	release chan struct{}
}

func (h *heldElevator) Remove(context.Context, string, string, string) error { return nil }
func (h *heldElevator) Scan(context.Context, string, string) (diskmodel.Scan, error) {
	close(h.started)
	<-h.release

	return diskmodel.Scan{Root: "old", Done: true}, nil
}

func TestAnOldWalkCannotMarkTheCurrentWalkFinished(t *testing.T) {
	t.Parallel()

	scanner := NewScanner(nil)
	scanner.generation = 2
	scanner.running.Store(true)
	scanner.finish(1)
	if !scanner.Running() {
		t.Fatal("an old walk cleared the current walk's running state")
	}
	scanner.finish(2)
	if scanner.Running() {
		t.Fatal("the current walk did not clear its running state")
	}
}

func TestAnOldWalkCannotRepopulateAClearedCache(t *testing.T) {
	t.Parallel()

	cache := newCache()
	stamp := measured{mtimeSec: 1, ctimeSec: 1}
	cache.begin(1, false)
	cache.store(1, key{dev: 1, ino: 1}, stamp, 10, worthKeeping)
	cache.begin(2, true)
	cache.store(1, key{dev: 1, ino: 1}, stamp, 20, worthKeeping)
	if _, ok := cache.lookup(2, key{dev: 1, ino: 1}, stamp); ok {
		t.Fatal("a canceled walk repopulated the cache after a full restart")
	}
}

func TestAnElevatedResultCannotReplaceANewerScan(t *testing.T) {
	t.Parallel()

	elevator := &heldElevator{started: make(chan struct{}), release: make(chan struct{})}
	scanner := NewScanner(nil)
	scanner.SetElevator(elevator)
	scanner.current = diskmodel.Scan{Root: "old", Done: true}

	done := make(chan error, 1)
	go func() { done <- scanner.RestartElevated(context.Background(), "old", "secret") }()
	<-elevator.started

	scanner.Start(t.Context(), t.TempDir())
	newer := scanner.Scan().Root
	close(elevator.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if got := scanner.Scan().Root; got != newer {
		t.Fatalf("old elevated result replaced newer root %q with %q", newer, got)
	}
}
