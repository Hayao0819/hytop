package conf

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/Hayao0819/hytop/internal/errors"
)

// Marshal serializes every setting, including defaults, for reproducible exports.
func Marshal(c Config) ([]byte, error) {
	out, err := toml.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(err, "writing the configuration")
	}

	return append([]byte("# written by hytop\n\n"), out...), nil
}

// Save replaces the destination atomically.
func Save(path string, c Config) error {
	out, err := Marshal(c)
	if err != nil {
		return err
	}

	temporary, err := stage(path, out)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary) }()

	if err := os.Rename(temporary, path); err != nil {
		return errors.Wrapf(err, "renaming onto %s", path)
	}

	return nil
}

func stage(path string, contents []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", errors.Wrapf(err, "creating %s", filepath.Dir(path))
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return "", errors.Wrapf(err, "creating a temporary file beside %s", path)
	}

	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporary.Name())
		}
	}()

	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()

		return "", errors.Wrapf(err, "writing %s", temporary.Name())
	}

	if err := temporary.Sync(); err != nil {
		return "", errors.Wrapf(err, "syncing %s", temporary.Name())
	}

	if err := temporary.Close(); err != nil {
		return "", errors.Wrapf(err, "closing %s", temporary.Name())
	}

	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		return "", errors.Wrapf(err, "setting the mode of %s", temporary.Name())
	}

	complete = true

	return temporary.Name(), nil
}
