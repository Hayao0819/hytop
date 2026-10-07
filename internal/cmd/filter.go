package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/errors"
)

func newFilterCmd() *cobra.Command {
	check := &cobra.Command{
		Use:           "filter [expression]",
		Short:         "Check a filter expression without starting the UI",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(),
					"fields:    "+strings.Join(filter.Fields(), " ")+"\n"+
						"relations: children descendants subtree ancestors siblings\n"+
						"operators: == != > < >= <= ~ ^= and or not")
				return err
			}

			expr, err := filter.Compile(args[0])
			if err != nil {
				return errors.Wrap(err, "parsing the expression")
			}

			if expr == nil {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "(empty: selects every process)")
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), expr.String())
			return err
		},
	}

	return check
}
