//go:build linux

// Package filesystem reads mounted filesystem usage.
package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"

	"github.com/Hayao0819/hytop/internal/collect"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/errors"
)

// virtual are the filesystems that hold no storage. They are excluded by type
// rather than by path, since where they are mounted is up to the distribution.
var virtual = map[string]bool{
	"autofs": true, "bpf": true, "binfmt_misc": true, "cgroup": true,
	"cgroup2": true, "configfs": true, "debugfs": true, "devpts": true,
	"devtmpfs": true, "efivarfs": true, "fuse.gvfsd-fuse": true,
	"fuse.portal": true, "fusectl": true, "hugetlbfs": true, "mqueue": true,
	"nsfs": true, "proc": true, "pstore": true, "ramfs": true, "rpc_pipefs": true,
	"securityfs": true, "selinuxfs": true, "squashfs": true, "sysfs": true,
	"tracefs": true,
}

// memoryBacked are the filesystems that live in RAM. They are still worth a row
// of their own — a full /tmp stops things working — but they are not capacity.
var memoryBacked = map[string]bool{"tmpfs": true, "devtmpfs": true, "ramfs": true}

type Collector struct {
	procRoot string
	mounts   string

	mu    sync.RWMutex
	found []diskmodel.Mount
}

func New(procRoot string) *Collector {
	return &Collector{procRoot: procRoot, mounts: filepath.Join(procRoot, "self", "mountinfo")}
}

func (c *Collector) Check() collect.Availability {
	_, err := os.Stat(c.mounts)

	return collect.KernelAvailability(err, "mount /proc")
}

// Mounts returns the latest filesystem snapshot.
func (c *Collector) Mounts() []diskmodel.Mount {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.found)
}

func (c *Collector) Collect(_ context.Context, now time.Time) ([]metric.Sample, error) {
	mounts, err := c.read()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.found = mounts
	c.mu.Unlock()

	return buildSamples(mounts, now, func(mount diskmodel.Mount) bool {
		return !memoryBacked[mount.FSType]
	}), nil
}

func (c *Collector) read() ([]diskmodel.Mount, error) {
	entries, err := readMountInfo(c.mounts)
	if err != nil {
		return nil, err
	}

	var (
		mounts []diskmodel.Mount
		seen   = map[string]bool{}
	)

	for _, entry := range entries {
		if virtual[entry.FSType] {
			continue
		}

		path := entry.Mountpoint
		if seen[path] || strings.HasPrefix(path, "/run/credentials/") {
			continue
		}

		seen[path] = true

		var stat unix.Statfs_t
		if err := unix.Statfs(path, &stat); err != nil {
			// A mount the user may not stat — another user's fuse mount, or
			// one that has gone away — is not a reason to fail the round.
			continue
		}

		if stat.Blocks == 0 {
			continue
		}

		unit := uint64(stat.Bsize)

		mounts = append(mounts, diskmodel.Mount{
			Path:       path,
			Device:     safe.Text(entry.Source),
			FSType:     safe.Text(entry.FSType),
			Opts:       safe.Text(options(entry.Options, entry.VFSOptions)),
			Total:      stat.Blocks * unit,
			Free:       stat.Bfree * unit,
			Available:  stat.Bavail * unit,
			Inodes:     stat.Files,
			InodesFree: stat.Ffree,
		})
	}

	sort.Slice(mounts, func(i, j int) bool { return mounts[i].Path < mounts[j].Path })

	return mounts, nil
}

// Boundaries reads every mount in the namespace without applying the capacity
// filters used by Collector. Directory traversal and deletion must also stop at
// virtual, zero-sized, and temporarily unstatable filesystems.
func Boundaries(procRoot string) ([]diskmodel.Mount, error) {
	path := filepath.Join(procRoot, "self", "mountinfo")
	entries, err := readMountInfo(path)
	if err != nil {
		return nil, err
	}

	mounts := make([]diskmodel.Mount, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		path := filepath.Clean(entry.Mountpoint)
		if seen[path] {
			continue
		}
		seen[path] = true
		mounts = append(mounts, diskmodel.Mount{Path: path})
	}

	sort.Slice(mounts, func(i, j int) bool { return mounts[i].Path < mounts[j].Path })

	return mounts, nil
}

func readMountInfo(path string) ([]*mountinfo.Info, error) {
	source, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %s", path)
	}
	defer func() { _ = source.Close() }()

	entries, err := mountinfo.GetMountsFromReader(source, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", path)
	}

	return entries, nil
}

func options(groups ...string) string {
	seen := map[string]bool{}
	var values []string
	for _, group := range groups {
		for option := range strings.SplitSeq(group, ",") {
			if option != "" && !seen[option] {
				seen[option] = true
				values = append(values, option)
			}
		}
	}
	sort.Strings(values)

	return strings.Join(values, ",")
}
