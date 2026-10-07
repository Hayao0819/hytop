//go:build !linux

package diskusage

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/errors"
)

type Elevate interface {
	Remove(context.Context, string, string, string) error
	Scan(context.Context, string, string) (diskmodel.Scan, error)
}

type Scanner struct {
	mu         sync.Mutex
	scan       diskmodel.Scan
	running    atomic.Bool
	cancel     context.CancelFunc
	generation uint64
	elevate    Elevate
	mounts     MountSource
}

func NewScanner(mounts MountSource) *Scanner {
	if mounts == nil {
		mounts = noMounts
	}
	return &Scanner{mounts: mounts}
}

func (s *Scanner) Scan() diskmodel.Scan                     { s.mu.Lock(); defer s.mu.Unlock(); return s.scan }
func (s *Scanner) Running() bool                            { return s.running.Load() }
func (s *Scanner) CanElevate() bool                         { return s.elevate != nil }
func (s *Scanner) SetElevator(e Elevate)                    { s.elevate = e }
func (s *Scanner) Start(ctx context.Context, root string)   { s.start(ctx, root) }
func (s *Scanner) Restart(ctx context.Context, root string) { s.start(ctx, root) }
func (s *Scanner) start(ctx context.Context, root string) {
	s.Stop()
	resolved, err := canonical(root)
	if err != nil {
		resolved, _ = filepath.Abs(root)
	}
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.generation++
	generation := s.generation
	s.cancel = cancel
	s.scan = diskmodel.Scan{Root: resolved}
	s.mu.Unlock()
	s.running.Store(true)
	go func() {
		scan, err := Measure(ctx, resolved, s.mounts)
		if err != nil {
			scan = diskmodel.Scan{Root: resolved, Done: true, Err: err.Error()}
		}
		s.mu.Lock()
		if generation == s.generation {
			s.scan = scan
			s.running.Store(false)
		}
		s.mu.Unlock()
	}()
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

func (s *Scanner) Remove(path string) error {
	scan := s.Scan()
	return RemoveInside(scan.Root, path, s.mounts)
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
	defer s.mu.Unlock()
	if mine == s.generation {
		s.generation++
		s.scan = scan
		s.running.Store(false)
	}
	return nil
}

func RemoveInside(root, path string, mounts MountSource) error {
	if mounts == nil {
		mounts = noMounts
	}
	rootAbs, err := canonical(root)
	if err != nil {
		return err
	}
	pathAbs, err := canonical(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return os.ErrPermission
	}
	all, err := mounts()
	if err != nil {
		return err
	}
	for _, mount := range all {
		mountAbs, _ := absolute(mount.Path)
		if mountAbs == pathAbs || strings.HasPrefix(mountAbs, pathAbs+string(os.PathSeparator)) {
			return os.ErrPermission
		}
	}
	rooted, err := os.OpenRoot(rootAbs)
	if err != nil {
		return err
	}
	defer func() { _ = rooted.Close() }()
	return rooted.RemoveAll(rel)
}

func Measure(ctx context.Context, root string, mounts MountSource) (diskmodel.Scan, error) {
	if mounts == nil {
		mounts = noMounts
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return diskmodel.Scan{}, err
	}
	blocked := map[string]bool{}
	all, err := mounts()
	if err != nil {
		return diskmodel.Scan{}, err
	}
	for _, mount := range all {
		path, _ := filepath.Abs(mount.Path)
		blocked[path] = true
	}
	result := diskmodel.Scan{Root: root, Done: true}
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return diskmodel.Scan{}, ctx.Err()
		default:
		}
		path := filepath.Join(root, entry.Name())
		item := diskmodel.Entry{Name: safe.Text(entry.Name()), Path: path, Dir: entry.IsDir()}
		abs, _ := filepath.Abs(path)
		if blocked[abs] {
			item.Mount = true
			result.Entries = append(result.Entries, item)
			continue
		}
		var problem error
		err := filepath.WalkDir(path, func(current string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if problem == nil {
					problem = errors.Wrapf(walkErr, "reading %s", current)
				}
				return nil
			}
			if current != path && blocked[current] && d.IsDir() {
				return filepath.SkipDir
			}
			info, e := d.Info()
			if e == nil {
				item.Size += uint64(info.Size())
				item.Items++
			} else if problem == nil {
				problem = errors.Wrapf(e, "reading %s", current)
			}
			return nil
		})
		if err != nil {
			return diskmodel.Scan{}, err
		}
		if result.Err == "" && problem != nil {
			result.Err = errors.Wrapf(problem, "scanning %s", root).Error()
		}
		result.Total += item.Size
		result.Items += item.Items
		result.Entries = append(result.Entries, item)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Size > result.Entries[j].Size })
	return result, nil
}
