//go:build linux

package diskusage

import (
	"sync"

	"golang.org/x/sys/unix"
)

// cache stores direct-file totals; subtree totals are never reused.
type cache struct {
	mu         sync.Mutex
	dirs       map[key]measured
	generation uint64

	reused int
}

type key struct {
	dev uint64
	ino uint64
}

type measured struct {
	files     uint64
	mtimeSec  int64
	ctimeSec  int64
	fileCount int32
	entries   int32
	mtimeNsec int32
	ctimeNsec int32
}

// Caching eight-entry directories kept benchmarked cache use below 5 MiB.
const worthKeeping = 8

func newCache() *cache { return &cache{dirs: map[key]measured{}} }

func identify(info unix.Stat_t) key { return key{dev: uint64(info.Dev), ino: info.Ino} }

func stamp(info unix.Stat_t, entries int) measured {
	return measured{
		mtimeSec:  int64(info.Mtim.Sec),
		mtimeNsec: int32(info.Mtim.Nsec),
		ctimeSec:  int64(info.Ctim.Sec),
		ctimeNsec: int32(info.Ctim.Nsec),
		entries:   int32(entries),
	}
}

func (m measured) same(other measured) bool {
	return m.mtimeSec == other.mtimeSec && m.mtimeNsec == other.mtimeNsec &&
		m.ctimeSec == other.ctimeSec && m.ctimeNsec == other.ctimeNsec &&
		m.entries == other.entries
}

func (c *cache) lookup(generation uint64, id key, now measured) (measured, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if generation != c.generation {
		return measured{}, false
	}

	stored, ok := c.dirs[id]
	if !ok || !stored.same(now) {
		return measured{}, false
	}

	c.reused++

	return stored, true
}

func (c *cache) store(generation uint64, id key, now measured, files uint64, count int) {
	if count < worthKeeping {
		return
	}

	now.files, now.fileCount = files, int32(count)

	c.mu.Lock()
	defer c.mu.Unlock()

	if generation != c.generation {
		return
	}

	c.dirs[id] = now
}

func (c *cache) begin(generation uint64, clear bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.generation = generation
	c.reused = 0
	if clear {
		c.dirs = map[key]measured{}
	}
}

func (c *cache) reuse(generation uint64) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if generation != c.generation {
		return 0
	}

	return c.reused
}
