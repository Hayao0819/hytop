//go:build linux

// Package diskusage scans directory trees and publishes incremental results.
package diskusage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

// Scanner manages one active directory scan.
type Scanner struct {
	mu      sync.Mutex
	current diskmodel.Scan
	cancel  context.CancelFunc

	// mounts defines traversal and deletion boundaries.
	mounts MountSource

	// generation rejects results from canceled scans that finish late.
	generation uint64

	cache *cache

	workers workers

	running atomic.Bool
	elevate Elevate
}

// Elevate performs the two operations that may need administrative access.
// Implementations must pass secrets through stdin, never argv or the environment.
type Elevate interface {
	Remove(context.Context, string, string, string) error
	Scan(context.Context, string, string) (diskmodel.Scan, error)
}

func NewScanner(mounts MountSource) *Scanner {
	if mounts == nil {
		mounts = noMounts
	}

	// Walks are I/O-bound, so allow more workers than processors.
	return &Scanner{
		mounts:  mounts,
		cache:   newCache(),
		workers: newWorkers(2 * runtime.GOMAXPROCS(0)),
	}
}

// Scan returns the latest incremental result.
func (s *Scanner) Scan() diskmodel.Scan {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.current
}

func (s *Scanner) Running() bool { return s.running.Load() }

func (s *Scanner) CanElevate() bool { return s.elevate != nil }

func (s *Scanner) SetElevator(e Elevate) { s.elevate = e }

// Start scans root and reuses valid direct-file totals.
func (s *Scanner) Start(ctx context.Context, root string) { s.start(ctx, root, false) }

// Restart discards cached measurements before scanning root.
func (s *Scanner) Restart(ctx context.Context, root string) { s.start(ctx, root, true) }

func (s *Scanner) start(ctx context.Context, root string, full bool) {
	s.Stop()

	absolute, err := canonical(root)
	if err != nil {
		absolute, _ = filepath.Abs(root)
	}

	walk, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.generation++
	mine := s.generation
	s.current = diskmodel.Scan{Root: absolute}
	s.cancel = cancel
	s.mu.Unlock()
	s.cache.begin(mine, full)

	s.running.Store(true)

	go s.walk(walk, absolute, mine)
}

func (s *Scanner) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.generation++
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.running.Store(false)
}

func sorted(entries []diskmodel.Entry) []diskmodel.Entry {
	out := slices.Clone(entries)

	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}

		return out[i].Name < out[j].Name
	})

	return out
}

func (s *Scanner) publish(entries []diskmodel.Entry, total uint64, items int, done bool, problem error, walk uint64) {
	reused := s.cache.reuse(walk)

	s.mu.Lock()
	defer s.mu.Unlock()

	if walk != s.generation {
		return
	}

	s.current.Entries, s.current.Total, s.current.Items = entries, total, items
	s.current.Done, s.current.Reused = done, reused
	if problem != nil {
		s.current.Err = errors.Wrapf(problem, "scanning %s", s.current.Root).Error()
	} else {
		s.current.Err = ""
	}
}

func (s *Scanner) fail(err error, walk uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if walk != s.generation {
		return
	}

	s.current.Err = errors.Wrapf(err, "scanning %s", s.current.Root).Error()
	s.current.Done = true
}

// Remove deletes a descendant of the scan root after validating mount boundaries.
func (s *Scanner) Remove(path string) error {
	return RemoveInside(s.Scan().Root, path, s.mounts)
}

// RemoveInside removes path only when it is below root and crosses no mount.
func RemoveInside(root, path string, mounts MountSource) error {
	if mounts == nil {
		mounts = noMounts
	}

	rootAbs, err := canonical(root)
	if err != nil {
		return errors.Wrapf(err, "resolving %s", root)
	}
	pathAbs, err := canonical(path)
	if err != nil {
		return errors.Wrapf(err, "resolving %s", path)
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) ||
		(len(rel) > 2 && rel[:3] == "../") {
		return errors.Newf("%s is not inside %s", pathAbs, rootAbs)
	}

	// os.RemoveAll can empty a nested mount before failing at its mount point.
	all, err := mounts()
	if err != nil {
		return errors.Wrap(err, "reading mount boundaries")
	}

	held := ""
	for _, mount := range all {
		mountPath, err := absolute(mount.Path)
		if err == nil && (mountPath == pathAbs || within(pathAbs, mountPath)) {
			held = mountPath
			break
		}
	}
	if held != "" {
		return errors.Newf("%s holds another filesystem, mounted at %s: unmount it first", path, held)
	}

	rooted, err := os.OpenRoot(rootAbs)
	if err != nil {
		return errors.Wrapf(err, "opening %s", rootAbs)
	}
	defer func() { _ = rooted.Close() }()

	if err := rooted.RemoveAll(rel); err != nil {
		return errors.Wrapf(err, "removing %s", pathAbs)
	}

	return nil
}

func (s *Scanner) RemoveElevated(ctx context.Context, path, password string) error {
	if s.elevate == nil {
		return errors.New("privilege elevation is unavailable")
	}

	return s.elevate.Remove(ctx, s.Scan().Root, path, password)
}

func (s *Scanner) RestartElevated(ctx context.Context, root, password string) error {
	if s.elevate == nil {
		return errors.New("privilege elevation is unavailable")
	}

	s.mu.Lock()
	mine := s.generation
	s.mu.Unlock()

	scan, err := s.elevate.Scan(ctx, root, password)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if mine != s.generation {
		s.mu.Unlock()

		return nil
	}
	cancel := s.cancel
	s.cancel = nil
	s.generation++
	s.current = scan
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.running.Store(false)

	return nil
}

// Measure performs one complete scan for the privileged helper.
func Measure(ctx context.Context, root string, mounts MountSource) (diskmodel.Scan, error) {
	s := NewScanner(mounts)
	s.Start(ctx, root)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		scan := s.Scan()
		if scan.Done {
			if scan.Err != "" {
				return scan, errors.New(scan.Err)
			}
			return scan, nil
		}

		select {
		case <-ctx.Done():
			return scan, ctx.Err()
		case <-ticker.C:
		}
	}
}
