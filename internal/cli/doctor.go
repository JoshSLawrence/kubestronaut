package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/session"
	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

// minDockerMemory is the smallest Docker memory that comfortably fits one
// two-node cluster.
const minDockerMemory = 6 << 30

func first(ctx context.Context, r execx.Runner, name string, args ...string) (string, error) {
	res, err := r.Run(ctx, execx.Cmd{Name: name, Args: args})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("exit %d: %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	lines := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	return strings.TrimSpace(lines[0]), nil
}

func (a *App) doctorCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check docker, kind, kubectl, memory and orphaned clusters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w, ctx, r := cmd.OutOrStdout(), cmd.Context(), a.runner()
			problems := 0
			report := func(ok bool, label, detail, remedy string) {
				if ok {
					fprintf(w, "  [ok]   %s: %s\n", label, detail)
					return
				}
				problems++
				fprintf(w, "  [FAIL] %s: %s -- %s\n", label, detail, remedy)
			}
			v, err := first(ctx, r, "docker", "version", "--format", "{{.Server.Version}}")
			report(err == nil, "docker", orElse(v, err), "start Docker Desktop (or the docker daemon) and retry")
			v, err = first(ctx, r, "kind", "version")
			report(err == nil, "kind", orElse(v, err), "install kind (mise install kind)")
			v, err = first(ctx, r, "kubectl", "version", "--client")
			report(err == nil, "kubectl", orElse(v, err), "install kubectl (mise install kubectl)")
			if mem, err := first(ctx, r, "docker", "info", "--format", "{{.MemTotal}}"); err == nil {
				n, _ := strconv.ParseInt(mem, 10, 64)
				report(n >= minDockerMemory, "docker memory", fmt.Sprintf("%.1f GiB", float64(n)/(1<<30)),
					"give Docker at least 6 GiB; the clusters need it")
			}

			store, err := a.store()
			if err != nil {
				return err
			}
			be := a.backendFor(store)
			clusters, err := be.List(ctx)
			if err != nil {
				report(false, "clusters", err.Error(), "check that docker and kind work")
				return doctorResult(problems)
			}
			owned := map[string]bool{}
			if st, err := store.Load(); err == nil {
				for _, c := range st.Clusters {
					owned[c.Name] = true
				}
			}
			for _, c := range clusters {
				if owned[c] {
					continue
				}
				if fix {
					ui.Step(w, "Deleting orphaned cluster %s", c)
					if err := be.Delete(ctx, c); err != nil {
						report(false, "orphan "+c, err.Error(), "delete it with: kind delete cluster --name "+c)
					}
					continue
				}
				report(false, "orphan cluster", c, "run `kubestronaut doctor --fix` or `kubestronaut stop --all`")
			}
			if _, err := store.Load(); err != nil && !errors.Is(err, session.ErrNoSession) {
				report(false, "state", err.Error(), "run `kubestronaut stop --all`")
			}
			if problems == 0 {
				fprintf(w, "All good.\n")
			}
			return doctorResult(problems)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "delete orphaned ks-* clusters")
	return cmd
}

func orElse(v string, err error) string {
	if err != nil {
		return err.Error()
	}
	return v
}

func doctorResult(problems int) error {
	if problems > 0 {
		return &ExitCodeError{Code: ExitError, Err: fmt.Errorf("%d problem(s) found", problems)}
	}
	return nil
}

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and detected tool versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w, ctx, r := cmd.OutOrStdout(), cmd.Context(), a.runner()
			fprintf(w, "kubestronaut %s\n", a.Version)
			for _, t := range []struct {
				name string
				args []string
			}{
				{"docker", []string{"version", "--format", "{{.Server.Version}}"}},
				{"kind", []string{"version"}},
				{"kubectl", []string{"version", "--client"}},
			} {
				v, err := first(ctx, r, t.name, t.args...)
				if err != nil {
					v = "not available"
				}
				fprintf(w, "%-8s %s\n", t.name+":", v)
			}
			return nil
		},
	}
}
