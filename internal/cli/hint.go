package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) hintCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "hint",
		Short: "Show the next hint (counted in your grade)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			hints, shown, total, err := l.Hint(all)
			if err != nil {
				return err
			}
			for i, h := range hints {
				n := shown
				if all {
					n = i + 1
				}
				fprintf(cmd.OutOrStdout(), "Hint %d/%d: %s\n", n, total, h)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "show every hint")
	return cmd
}

func (a *App) solutionCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "solution",
		Short: "Show the reference solution (flagged in your grade)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := confirm(cmd, yes, "Viewing the solution is recorded in your grade. Continue?"); err != nil {
				return err
			}
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			text, err := l.Solution()
			if err != nil {
				return err
			}
			fprintf(cmd.OutOrStdout(), "%s", text)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask for confirmation")
	return cmd
}
