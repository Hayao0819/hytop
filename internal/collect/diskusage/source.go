package diskusage

import (
	"path/filepath"

	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
)

// MountSource returns every mount in the current namespace. Callers use a
// fresh reading for each scan and delete because mount boundaries are safety
// information, not display data that may be cached or filtered.
type MountSource func() ([]diskmodel.Mount, error)

func noMounts() ([]diskmodel.Mount, error) { return nil, nil }

func canonical(path string) (string, error) {
	absolute, err := absolute(path)
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(absolute)
}

func absolute(path string) (string, error) { return filepath.Abs(filepath.Clean(path)) }
