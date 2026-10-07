package conf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	koanftoml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/pelletier/go-toml/v2"

	"github.com/Hayao0819/hytop/internal/errors"
)

// Sources lists configuration files in override order.
type Sources struct {
	System  string
	User    string
	Profile string
	Extra   string
}

const (
	name    = "hytop"
	file    = "config.toml"
	profile = "profiles"
)

// Discover resolves the system, user, explicit, and profile configuration paths.
func Discover(explicit, profileName string, env func(string) string) Sources {
	if env == nil {
		env = os.Getenv
	}

	if explicit != "" {
		return Sources{Extra: explicit, Profile: profilePath(filepath.Dir(explicit), profileName)}
	}

	dir := UserDir(env)
	if dir == "" {
		// Never fall back to a relative configuration path.
		return Sources{System: systemConfigPath()}
	}

	return Sources{
		System:  systemConfigPath(),
		User:    filepath.Join(dir, file),
		Profile: profilePath(dir, profileName),
	}
}

// UserDir follows the XDG basedir spec, whose fallback is ~/.config.
func UserDir(env func(string) string) string {
	if env == nil {
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			return ""
		}

		return filepath.Join(dir, name)
	}

	if base := env("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, name)
	}

	if home := env("HOME"); home != "" {
		return filepath.Join(home, ".config", name)
	}

	return ""
}

func profilePath(dir, profileName string) string {
	if dir == "" || profileName == "" {
		return ""
	}

	return filepath.Join(dir, profile, profileName+".toml")
}

// Files returns the configured layers in override order.
func (s Sources) Files() []string {
	paths := make([]string, 0, 4)

	for _, path := range []string{s.System, s.User, s.Extra, s.Profile} {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// Load reads every layer over Default and validates the result. Missing search
// paths are skipped, while explicitly named configurations and profiles fail.
func (s Sources) Load() (Config, error) { return s.load(nil) }

// LoadWith applies command-line overrides after the file layers.
func (s Sources) LoadWith(override func(*Config)) (Config, error) { return s.load(override) }

func (s Sources) load(override func(*Config)) (Config, error) {
	settings := Default()
	layers := koanf.New(".")

	for _, path := range s.Files() {
		source, err := os.ReadFile(path)
		if err != nil {
			// Implicit search paths are optional; explicit files are required.
			if os.IsNotExist(err) && path != s.Extra && path != s.Profile {
				continue
			}

			return Config{}, errors.Wrapf(err, "reading %s", path)
		}

		if err := validateSource(source); err != nil {
			return Config{}, errors.Newf("%s (%s)", err, path)
		}

		if err := layers.Load(rawbytes.Provider(source), koanftoml.Parser()); err != nil {
			return Config{}, errors.Newf("%s (%s)", explain(err), path)
		}
	}

	if err := layers.UnmarshalWithConf("", &settings, koanf.UnmarshalConf{Tag: "toml"}); err != nil {
		return Config{}, errors.Wrap(err, "decoding configuration")
	}

	if override != nil {
		override(&settings)
	}

	if err := settings.Validate(); err != nil {
		return Config{}, err
	}

	return settings, nil
}

// validateSource preserves precise unknown-key and source-position errors before
// koanf merges the layer into the implementation-independent Config value.
func validateSource(source []byte) error {
	decoder := toml.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&Config{}); err != nil {
		return explain(err)
	}

	return nil
}

func explain(err error) error {
	var unknown *toml.StrictMissingError
	if errors.As(err, &unknown) && len(unknown.Errors) > 0 {
		first := unknown.Errors[0]
		row, column := first.Position()

		return errors.Newf("line %d column %d: unknown setting %q",
			row, column, strings.Join(first.Key(), "."))
	}

	var broken *toml.DecodeError
	if errors.As(err, &broken) {
		row, column := broken.Position()

		return errors.Newf("line %d column %d: %s", row, column, broken.Error())
	}

	return err
}
