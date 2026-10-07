//go:build linux

package diskusage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/errors"
)

// boundaries uses mount points instead of device IDs so btrfs subvolumes stay
// within the scan while separately mounted filesystems do not.
func (s *Scanner) boundaries(root string) (map[string]bool, error) {
	stop := map[string]bool{}

	mounts, err := s.mounts()
	if err != nil {
		return nil, errors.Wrap(err, "reading mount boundaries")
	}

	for _, mount := range mounts {
		path, err := absolute(mount.Path)
		if err != nil {
			continue
		}

		if path == root {
			continue
		}

		if within(root, path) {
			stop[path] = true
		}
	}

	return stop, nil
}

func entries(path string) ([]os.DirEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = dir.Close() }()

	return dir.ReadDir(-1)
}

// take is non-blocking because workers may wait for descendant scans.
type workers chan struct{}

func newWorkers(n int) workers { return make(workers, max(1, n)) }

func (w workers) take() bool {
	select {
	case w <- struct{}{}:
		return true
	default:
		return false
	}
}

func (w workers) give() { <-w }

func within(root, path string) bool {
	if root == "/" {
		return path != "/"
	}

	return strings.HasPrefix(path, root+"/")
}

func (s *Scanner) walk(ctx context.Context, root string, generation uint64) {
	defer s.finish(generation)

	children, err := os.ReadDir(root)
	if err != nil {
		s.fail(err, generation)

		return
	}

	stop, err := s.boundaries(root)
	if err != nil {
		s.fail(err, generation)

		return
	}

	var (
		entries = make([]diskmodel.Entry, 0, len(children))
		total   uint64
		items   int
		last    = time.Now()
		problem error
	)

	for _, child := range children {
		if ctx.Err() != nil {
			return
		}

		path := filepath.Join(root, child.Name())

		entry := diskmodel.Entry{Name: safe.Text(child.Name()), Path: path, Dir: child.IsDir()}

		switch {
		case stop[path]:
			entry.Dir, entry.Mount = true, true

		case child.IsDir():
			var err error
			entry.Size, entry.Items, err = s.size(ctx, path, stop, generation)
			if problem == nil && err != nil {
				problem = err
			}

		default:
			if used, err := blocks(path); err == nil {
				entry.Size, entry.Items = used, 1
			} else if problem == nil {
				problem = err
			}
		}

		total += entry.Size
		items += entry.Items

		entries = append(entries, entry)

		// Bound publication and lock traffic in wide directories.
		if time.Since(last) > 250*time.Millisecond {
			last = time.Now()

			s.publish(sorted(entries), total, items, false, problem, generation)
		}
	}

	if ctx.Err() != nil {
		return
	}

	s.publish(sorted(entries), total, items, true, problem, generation)
}

func (s *Scanner) finish(generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation == s.generation {
		s.running.Store(false)
	}
}

// size reuses only direct-file totals and returns the first traversal error.
func (s *Scanner) size(ctx context.Context, root string, stop map[string]bool, generation uint64) (uint64, int, error) {
	var info unix.Stat_t

	if err := unix.Lstat(root, &info); err != nil {
		return 0, 0, errors.Wrapf(err, "reading %s", root)
	}

	children, err := entries(root)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "reading %s", root)
	}

	var (
		id            = identify(info)
		now           = stamp(info, len(children))
		cached, reuse = s.cache.lookup(generation, id, now)
	)

	var (
		files   uint64
		count   int
		problem error

		// Descendant scans add their totals concurrently.
		mu    sync.Mutex
		group sync.WaitGroup
		under uint64
		below int
	)

	if reuse {
		// Directory timestamps cannot detect in-place file growth.
		files, count = cached.files, int(cached.fileCount)
	}

	add := func(size uint64, items int, err error) {
		mu.Lock()
		defer mu.Unlock()

		under += size
		below += items

		if problem == nil && err != nil {
			problem = err
		}
	}

	for _, child := range children {
		if ctx.Err() != nil {
			group.Wait()

			return files + under, count + below, ctx.Err()
		}

		path := filepath.Join(root, child.Name())

		switch {
		case stop[path]:

		case child.IsDir():
			// Blocking for a worker can deadlock behind an ancestor's slot.
			if s.workers.take() {
				group.Go(func() {
					defer s.workers.give()

					add(s.size(ctx, path, stop, generation))
				})

				continue
			}

			add(s.size(ctx, path, stop, generation))

		case reuse:

		default:
			// Include links and special files in allocated-block totals.
			info, err := stat(path)
			if err != nil {
				if problem == nil {
					problem = err
				}
				continue
			}

			files += used(info)
			count++
		}
	}

	group.Wait()

	if problem == nil && !reuse {
		s.cache.store(generation, id, now, files, count)
	}

	return files + under, count + below, problem
}

// blocks returns allocated size rather than apparent size.
func blocks(path string) (uint64, error) {
	info, err := stat(path)
	if err != nil {
		return 0, err
	}

	return used(info), nil
}

func stat(path string) (info unix.Stat_t, err error) {
	if err := unix.Lstat(path, &info); err != nil {
		return info, errors.Wrapf(err, "reading %s", path)
	}

	return info, nil
}

// st_blocks is always expressed in 512-byte units.
func used(info unix.Stat_t) uint64 { return uint64(info.Blocks) * 512 }
