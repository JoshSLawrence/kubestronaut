package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

func (a *App) gradeCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "grade",
		Short: "Run the checks and print the score",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			rep, err := l.Grade(cmd.Context())
			if err != nil {
				return err
			}
			if asJSON {
				if err := ui.JSON(cmd.OutOrStdout(), rep); err != nil {
					return err
				}
			} else {
				ui.GradeText(cmd.OutOrStdout(), rep, a.Verbose)
			}
			switch {
			case rep.AllErrored():
				return &ExitCodeError{Code: ExitError, Err: errors.New("every check hit an infrastructure error, so this grade says nothing about your work: run `kubestronaut doctor` and check that the cluster is running")}
			case !rep.Passed:
				return &ExitCodeError{Code: ExitNotPassed}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
