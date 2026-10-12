package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/ui"
	"github.com/JoshSLawrence/kubestronaut/internal/validate"
)

func (a *App) validateCmd() *cobra.Command {
	var all, e2e, keep bool
	var version, image string
	cmd := &cobra.Command{
		Use:   "validate [<id>...|--all] [--e2e]",
		Short: "Validate scenarios statically, or prove them end to end",
		Long: "Without --e2e, check scenario files: schema, files, ids, nodes and check definitions.\n" +
			"With --e2e, also create a real cluster, require the checks to fail, run the\n" +
			"solution and require a 100% grade. Stop any active session first.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return errors.New("name at least one scenario id, or pass --all")
			}
			if all && len(args) > 0 {
				return errors.New("pass scenario ids or --all, not both")
			}
			cat := a.catalog()
			w := cmd.OutOrStdout()
			ids := args
			errs := validate.Static(cat, ids)
			for _, e := range errs {
				fprintf(cmd.ErrOrStderr(), "invalid: %v\n", e)
			}
			if len(errs) > 0 {
				return &ExitCodeError{Code: ExitNotPassed, Err: fmt.Errorf("%d validation problem(s) found", len(errs))}
			}
			if all {
				for _, s := range cat.Scenarios {
					ids = append(ids, s.ID)
				}
			}
			for _, id := range ids {
				if s, err := cat.Find(id); err == nil {
					fprintf(w, "ok    %s\n", s.ID)
				}
			}
			if !e2e {
				return nil
			}

			store, err := a.store()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			failed := 0
			for _, id := range ids {
				s, _ := cat.Find(id)
				if s.Metadata.Experimental && !all {
					ui.Warn(w, "%s is experimental", s.ID)
				}
				ui.Step(w, "End-to-end validation of %s", s.ID)
				res := validate.E2E(ctx, validate.Options{
					Catalog: cat, Backend: a.backendFor(store), Runner: a.runner(), Binary: a.binary(),
					HomeParent: validate.HomeParent(store.Root), Out: w, Verbose: a.Verbose, Keep: keep,
					K8sVersion: version, NodeImage: image,
				}, id)
				validate.Print(w, res)
				if !res.OK() {
					failed++
				}
				if ctx.Err() != nil {
					break
				}
			}
			if failed > 0 {
				return &ExitCodeError{Code: ExitNotPassed, Err: fmt.Errorf("%d of %d scenario(s) failed end-to-end validation", failed, len(ids))}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "validate every scenario")
	cmd.Flags().BoolVar(&e2e, "e2e", false, "also run each scenario against a real cluster")
	cmd.Flags().BoolVar(&keep, "keep", false, "keep the cluster after an end-to-end run, for debugging")
	cmd.Flags().StringVar(&version, "k8s-version", "", "Kubernetes version for end-to-end runs")
	cmd.Flags().StringVar(&image, "node-image", "", "raw kind node image for end-to-end runs")
	return cmd
}
