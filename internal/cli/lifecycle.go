package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/lab"
	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

func (a *App) startCmd() *cobra.Command {
	var o lab.StartOptions
	cmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Create a cluster, run setup and print the task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			return l.Start(cmd.Context(), args[0], o)
		},
	}
	cmd.Flags().StringVar(&o.K8sVersion, "k8s-version", "", "Kubernetes version, e.g. v1.36.4")
	cmd.Flags().StringVar(&o.NodeImage, "node-image", "", "raw kind node image, overrides --k8s-version")
	cmd.Flags().BoolVar(&o.Force, "force", false, "stop an active session first")
	cmd.Flags().BoolVar(&o.KeepOnFailure, "keep-on-failure", false, "keep the cluster if start fails, for debugging")
	return cmd
}

func (a *App) statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			s, err := l.Status()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fprintf(w, "Scenario:    %s (%s)\n", s.Scenario, s.Title)
			fprintf(w, "Cluster:     %s (%s)\n", s.Cluster, s.Version)
			fprintf(w, "Context:     %s\n", s.Context)
			var nodes []string
			for _, n := range s.Nodes {
				nodes = append(nodes, n.Alias)
			}
			fprintf(w, "Nodes:       %s\n", strings.Join(nodes, ", "))
			elapsed := "Elapsed:     " + ui.Duration(s.Elapsed)
			if s.TimeLimit > 0 {
				elapsed += " of " + ui.Duration(s.TimeLimit) + " suggested"
				if s.Elapsed > s.TimeLimit {
					elapsed += " (over)"
				}
			}
			fprintf(w, "%s\n", elapsed)
			fprintf(w, "Hints used:  %d\n", s.HintsUsed)
			if s.Solution {
				fprintf(w, "Solution:    viewed\n")
			}
			if s.LastGrade != nil {
				fprintf(w, "Last grade:  %d/%d (%s)\n", s.LastGrade.Earned, s.LastGrade.Possible, ui.Percent(s.LastGrade.Percent))
			}
			fprintf(w, "Kubeconfig:  %s\n", s.Kubeconfig)
			return nil
		},
	}
}

func (a *App) promptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prompt",
		Short: "Print the task prompt again",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			p, err := l.Prompt()
			if err != nil {
				return err
			}
			fprintf(cmd.OutOrStdout(), "%s", p)
			return nil
		},
	}
}

func (a *App) envCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Print the export line for the lab kubeconfig (use with eval)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			if _, err := l.Status(); err != nil {
				return err
			}
			// KUBECACHEDIR keeps kubectl's discovery cache out of ~/.kube.
			fprintf(cmd.OutOrStdout(), "export KUBECONFIG=%s\nexport KUBECACHEDIR=%s\n",
				l.Store.KubeconfigPath(), filepath.Join(l.Store.Root, "cache"))
			return nil
		},
	}
}

func (a *App) stopCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Delete the session's clusters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			return l.Stop(cmd.Context(), all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "also delete every orphaned ks-* cluster")
	return cmd
}

func (a *App) resetCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Recreate the cluster and rerun setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := confirm(cmd, yes, "Reset deletes the cluster and your work in it. Continue?"); err != nil {
				return err
			}
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			return l.Reset(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask for confirmation")
	return cmd
}

func (a *App) shellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Start your shell with KUBECONFIG set to the lab",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			if _, err := l.Status(); err != nil {
				return err
			}
			sh := os.Getenv("SHELL")
			if sh == "" {
				return errors.New("$SHELL is not set: export SHELL or use `eval \"$(kubestronaut env)\"` instead")
			}
			return a.runner().Interactive(cmd.Context(), execxCmd(sh, nil, "KUBECONFIG="+l.Store.KubeconfigPath(), "KUBECACHEDIR="+filepath.Join(l.Store.Root, "cache")))
		},
	}
}
