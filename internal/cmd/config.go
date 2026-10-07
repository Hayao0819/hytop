package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/errors"
)

func newConfigCmd() *cobra.Command {
	settings := &cobra.Command{
		Use:           "config",
		Short:         "Show or write hytop's settings",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	settings.AddCommand(newConfigShowCmd(), newConfigPathsCmd(), newConfigWriteCmd())

	return settings
}

// sources rebuilds the search from the root command's flags, so `hytop config`
// reads exactly what `hytop` would.
func sources(cmd *cobra.Command) conf.Sources {
	path, _ := cmd.Flags().GetString("config")
	profile, _ := cmd.Flags().GetString("profile")

	return conf.Discover(path, profile, nil)
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "show",
		Short:         "Print the settings every layer adds up to",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings, err := resolvedConfig(cmd)
			if err != nil {
				return err
			}

			out, err := conf.Marshal(settings)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(out)

			return err
		},
	}
}

func newConfigPathsCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "paths",
		Short:         "List the files that are read, in order",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, path := range sources(cmd).Files() {
				state := "missing"
				if _, err := os.Stat(path); err == nil {
					state = "read"
				}

				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-8s %s\n", state, path); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func newConfigWriteCmd() *cobra.Command {
	var force bool

	write := &cobra.Command{
		Use:           "write",
		Short:         "Write the current settings to the user's file",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("config")
			layers := sources(cmd)
			if path != "" {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					layers.Extra = ""
				} else if err != nil {
					return errors.Wrapf(err, "reading %s", path)
				}
			}

			settings, err := resolvedConfigFrom(cmd, layers)
			if err != nil {
				return err
			}

			if path == "" {
				dir := conf.UserDir(nil)
				if dir == "" {
					return errors.New("cannot find a user configuration directory; set HOME or XDG_CONFIG_HOME, or pass --config")
				}
				path = filepath.Join(dir, "config.toml")
			}

			if _, err := os.Stat(path); err == nil && !force {
				return errors.Newf("%s is already there; pass --force to replace it", path)
			}

			if err := conf.Save(path, settings); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "wrote "+path)
			return err
		},
	}

	write.Flags().BoolVar(&force, "force", false, "replace the file if it is already there")

	return write
}

func resolvedConfig(cmd *cobra.Command) (conf.Config, error) {
	return resolvedConfigFrom(cmd, sources(cmd))
}

func resolvedConfigFrom(cmd *cobra.Command, sources conf.Sources) (conf.Config, error) {
	opts := flagged(cmd)
	settings, err := sources.LoadWith(overrides(cmd, opts))
	if err != nil {
		return conf.Config{}, err
	}

	registry, err := seriesRegistry()
	if err != nil {
		return conf.Config{}, err
	}
	if err := settings.ValidateSeries(registry); err != nil {
		return conf.Config{}, err
	}

	return settings, nil
}
