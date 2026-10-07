package conf

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Hayao0819/hytop/internal/errors"
)

// Watcher retains the last valid configuration when a reload fails.
type Watcher struct {
	mu         sync.RWMutex
	sources    Sources
	override   func(*Config)
	current    Config
	problem    string
	generation uint64
	signature  uint64

	validators []func(Config) error
}

// NewWatcher loads the initial configuration and reapplies override on reload.
func NewWatcher(sources Sources, override func(*Config)) (*Watcher, error) {
	signature := sources.signature()
	config, err := sources.load(override)
	if err != nil {
		return nil, err
	}

	return &Watcher{
		sources: sources, override: override, current: config.Clone(), signature: signature,
	}, nil
}

func (w *Watcher) Config() Config {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.current.Clone()
}

// Generation counts successful reloads.
func (w *Watcher) Generation() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.generation
}

// Problem returns the latest reload error, if any.
func (w *Watcher) Problem() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.problem
}

// AddValidator validates the current value before registering a reload validator.
func (w *Watcher) AddValidator(validator func(Config) error) error {
	if validator == nil {
		return nil
	}

	for {
		w.mu.RLock()
		current, generation := w.current.Clone(), w.generation
		w.mu.RUnlock()

		if err := validator(current); err != nil {
			return err
		}

		w.mu.Lock()
		if generation != w.generation {
			w.mu.Unlock()

			continue
		}
		w.validators = append(w.validators, validator)
		w.mu.Unlock()

		return nil
	}
}

// Run watches directories so atomic file replacement remains observable.
func (w *Watcher) Run(ctx context.Context) error {
	files, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "watching the configuration")
	}

	session := newWatchSession(w, files)
	defer session.close()
	session.addDirectories()

	poll := time.NewTicker(time.Second)
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-files.Events:
			if !ok {
				return nil
			}
			session.inspect(event.Name)

		case <-poll.C:
			session.inspect("")

		case <-session.reloadReady:
			session.reload()

		case err, ok := <-files.Errors:
			if !ok {
				return nil
			}

			w.fail(err.Error())
		}
	}
}

const (
	reloadDelay    = 10 * time.Millisecond
	maxReloadDelay = 100 * time.Millisecond
)

type watchSession struct {
	owner        *Watcher
	files        *fsnotify.Watcher
	targets      map[string]bool
	watched      map[string]bool
	reloadTimer  *time.Timer
	reloadReady  <-chan time.Time
	pendingSince time.Time
}

func newWatchSession(owner *Watcher, files *fsnotify.Watcher) *watchSession {
	targets := make(map[string]bool, len(owner.sources.Files()))
	for _, path := range owner.sources.Files() {
		targets[filepath.Clean(path)] = true
	}

	return &watchSession{owner: owner, files: files, targets: targets, watched: map[string]bool{}}
}

func (s *watchSession) close() {
	if s.reloadTimer != nil {
		s.reloadTimer.Stop()
	}
	_ = s.files.Close()
}

func (s *watchSession) addDirectories() {
	for path := range s.targets {
		dir := nearestDirectory(filepath.Dir(path))
		if dir == "" || s.watched[dir] {
			continue
		}
		if err := s.files.Add(dir); err == nil {
			s.watched[dir] = true
		}
	}
}

func (s *watchSession) inspect(path string) {
	s.addDirectories()
	path = filepath.Clean(path)
	if path != "." && !s.targets[path] && !ancestorOfSource(path, s.targets) {
		return
	}
	if s.owner.changed() {
		s.schedule()
	}
}

func (s *watchSession) schedule() {
	now := time.Now()
	if s.reloadReady != nil && now.Sub(s.pendingSince) >= maxReloadDelay {
		return
	}
	if s.reloadTimer == nil {
		s.reloadTimer = time.NewTimer(reloadDelay)
	} else {
		if !s.reloadTimer.Stop() {
			select {
			case <-s.reloadTimer.C:
			default:
			}
		}
		s.reloadTimer.Reset(reloadDelay)
	}
	if s.reloadReady == nil {
		s.pendingSince = now
	}
	s.reloadReady = s.reloadTimer.C
}

func (s *watchSession) reload() {
	s.reloadReady = nil
	s.pendingSince = time.Time{}
	s.owner.reload()
}

func nearestDirectory(path string) string {
	for {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func ancestorOfSource(path string, sources map[string]bool) bool {
	prefix := filepath.Clean(path) + string(os.PathSeparator)
	for source := range sources {
		if strings.HasPrefix(filepath.Clean(source), prefix) {
			return true
		}
	}
	return false
}

func (s Sources) signature() uint64 {
	digest := fnv.New64a()
	write := func(value []byte) {
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write(value)
	}

	for _, path := range s.Files() {
		write([]byte(path))
		contents, err := os.ReadFile(path)
		if err != nil {
			write([]byte(err.Error()))
		} else {
			write(contents)
		}
	}

	return digest.Sum64()
}

func (w *Watcher) changed() bool {
	signature := w.sources.signature()

	w.mu.Lock()
	defer w.mu.Unlock()
	if signature == w.signature {
		return false
	}

	w.signature = signature

	return true
}

// Reload re-reads every layer. It is exported so a key can ask for it without
// waiting on the filesystem.
func (w *Watcher) Reload() {
	w.changed()
	w.reload()
}

func (w *Watcher) reload() {
	w.mu.RLock()
	override := w.override
	w.mu.RUnlock()

	config, err := w.sources.load(override)
	if err != nil {
		w.fail(err.Error())

		return
	}

	for {
		w.mu.RLock()
		validators := slices.Clone(w.validators)
		w.mu.RUnlock()

		for _, validator := range validators {
			if err := validator(config.Clone()); err != nil {
				w.fail(err.Error())

				return
			}
		}

		w.mu.Lock()
		if len(validators) != len(w.validators) {
			w.mu.Unlock()

			continue
		}

		w.current, w.problem = config.Clone(), ""
		w.generation++
		w.mu.Unlock()

		return
	}
}

func (w *Watcher) fail(reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.problem = "config: " + reason + " — keeping the last good one"
}
