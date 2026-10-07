package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/ui/keymap"
)

func newKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "keys",
		Short:         "List every binding and what it is bound to",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings, err := sources(cmd).Load()
			if err != nil {
				return err
			}

			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			for _, binding := range settings.Keymap().All() {
				if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n",
					keymap.Name(binding.Scope, binding.Action),
					strings.Join(binding.Keys, " "),
					binding.What); err != nil {
					return err
				}
			}

			return out.Flush()
		},
	}
}
