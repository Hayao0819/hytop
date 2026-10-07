// Package store holds collected data and persistent view state.
package store

import (
	"slices"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

// Store satisfies collect.Sink without importing collect: the interface is
// declared where it is used.
type Store struct {
	mu          sync.RWMutex
	resolutions []metric.Resolution
	series      map[series.Key]*metric.Series
	keys        []series.Key
	procs       *procmodel.Snapshot
	facts       map[string]string
	factOwners  map[string]string
	sourceFacts map[string]map[string]struct{}
	generation  uint64
	retention   time.Duration
	nextSweep   time.Time
}

func New(resolutions []metric.Resolution) *Store {
	if len(resolutions) == 0 {
		resolutions = metric.DefaultResolutions()
	}

	var retention time.Duration
	for _, resolution := range resolutions {
		retention = max(retention, resolution.Retention)
	}

	return &Store{
		resolutions: resolutions,
		series:      make(map[series.Key]*metric.Series),
		procs:       procmodel.NewSnapshot(nil),
		facts:       make(map[string]string),
		factOwners:  make(map[string]string),
		sourceFacts: make(map[string]map[string]struct{}),
		retention:   retention,
	}
}

func (s *Store) WriteSamples(samples []metric.Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newest time.Time

	for _, sample := range samples {
		if sample.Time.After(newest) {
			newest = sample.Time
		}

		history, ok := s.series[sample.Key]
		if !ok {
			history = metric.NewSeries(s.resolutions)
			s.series[sample.Key] = history
			s.keys = append(s.keys, sample.Key)

			slices.Sort(s.keys)
		}

		history.Push(sample.Time, sample.Value)
	}

	s.prune(newest)

	s.generation++
}

// prune removes expired dynamic-device series before their keys accumulate.
func (s *Store) prune(now time.Time) {
	if now.IsZero() || s.retention <= 0 || now.Before(s.nextSweep) {
		return
	}

	interval := min(time.Minute, max(time.Second, s.retention/4))
	s.nextSweep = now.Add(interval)
	cutoff := now.Add(-s.retention)

	kept := s.keys[:0]
	for _, key := range s.keys {
		history := s.series[key]
		last, ok := history.Last()
		if !ok || last.Time.Before(cutoff) {
			delete(s.series, key)

			continue
		}

		kept = append(kept, key)
	}

	s.keys = kept
}

func (s *Store) WriteProcesses(procs []procmodel.Process) {
	snapshot := procmodel.NewSnapshot(procs)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.procs = snapshot
	s.generation++
}

func (s *Store) WriteFacts(facts map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for key, value := range facts {
		value = safe.Text(value)
		if previous, ok := s.facts[key]; !ok || previous != value {
			changed = true
		}
		s.facts[key] = value

		if owner, ok := s.factOwners[key]; ok {
			delete(s.factOwners, key)
			delete(s.sourceFacts[owner], key)
			if len(s.sourceFacts[owner]) == 0 {
				delete(s.sourceFacts, owner)
			}
		}
	}

	if changed {
		s.generation++
	}
}

func (s *Store) ReplaceFacts(source string, facts map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	owned := s.sourceFacts[source]
	if owned == nil && len(facts) > 0 {
		owned = make(map[string]struct{}, len(facts))
		s.sourceFacts[source] = owned
	}
	changed := false

	for key := range owned {
		if _, ok := facts[key]; ok {
			continue
		}
		if s.factOwners[key] == source {
			delete(s.facts, key)
			delete(s.factOwners, key)
			changed = true
		}
		delete(owned, key)
	}

	for key, value := range facts {
		value = safe.Text(value)
		if previous, ok := s.facts[key]; !ok || previous != value {
			changed = true
		}
		s.facts[key] = value

		if previous := s.factOwners[key]; previous != "" && previous != source {
			delete(s.sourceFacts[previous], key)
			if len(s.sourceFacts[previous]) == 0 {
				delete(s.sourceFacts, previous)
			}
		}
		s.factOwners[key] = source
		owned[key] = struct{}{}
	}

	if len(owned) == 0 {
		delete(s.sourceFacts, source)
	}

	if changed {
		s.generation++
	}
}

// Fact is a text reading that barely changes, such as a CPU model.
func (s *Store) Fact(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.facts[key]

	return value, ok
}

// Generation changes whenever anything is written, which is what a widget uses
// as its memo key.
func (s *Store) Generation() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.generation
}

// Processes is safe to hold: a write replaces the snapshot rather than editing
// it.
func (s *Store) Processes() *procmodel.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.procs
}

func (s *Store) Keys() []series.Key {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Clone(s.keys)
}

func (s *Store) Window(key series.Key, span time.Duration) []metric.Point {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, ok := s.series[key]
	if !ok {
		return nil
	}

	return history.Window(time.Now(), span)
}

func (s *Store) Last(key series.Key) (metric.Point, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, ok := s.series[key]
	if !ok {
		return metric.Point{}, false
	}

	return history.Last()
}
