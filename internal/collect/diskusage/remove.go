package diskusage

import (
	"os"
	"path/filepath"

	"github.com/Hayao0819/hytop/internal/errors"
)

// RemoveInside removes path only when it is below root and crosses no mount.
func RemoveInside(root, path string, mounts MountSource) error {
	if mounts == nil {
		mounts = noMounts
	}
	rootAbs, err := canonical(root)
	if err != nil {
		return errors.Wrapf(err, "resolving %s", root)
	}
	pathAbs, err := absolute(path)
	if err != nil {
		return errors.Wrapf(err, "resolving %s", path)
	}
	parent, err := canonical(filepath.Dir(pathAbs))
	if err != nil {
		return errors.Wrapf(err, "resolving parent of %s", path)
	}
	pathAbs = filepath.Join(parent, filepath.Base(pathAbs))
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return errors.Newf("%s is not inside %s", pathAbs, rootAbs)
	}

	// RemoveAll can empty a nested mount before failing at its mount point.
	all, err := mounts()
	if err != nil {
		return errors.Wrap(err, "reading mount boundaries")
	}
	for _, mount := range all {
		mountPath, err := absolute(mount.Path)
		if err != nil {
			return errors.Wrapf(err, "resolving mount %s", mount.Path)
		}
		inside, err := filepath.Rel(pathAbs, mountPath)
		if err == nil && filepath.IsLocal(inside) {
			return errors.Newf("%s holds another filesystem, mounted at %s: unmount it first", path, mountPath)
		}
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
