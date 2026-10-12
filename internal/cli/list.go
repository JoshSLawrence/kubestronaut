package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

type listEntry struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Domain     string   `json:"domain"`
	Difficulty string   `json:"difficulty,omitempty"`
	Nodes      int      `json:"nodes"`
	TimeLimit  string   `json:"timeLimit,omitempty"`
	Weight     int      `json:"weight,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func entryOf(s *scenario.Scenario) listEntry {
	e := listEntry{
		ID: s.ID, Title: s.Metadata.Title, Domain: s.Metadata.Domain, Difficulty: s.Metadata.Difficulty,
		Nodes: s.Spec.Cluster.CPs() + s.Spec.Cluster.Workers, Weight: s.Spec.Weight, Tags: s.Metadata.Tags,
	}
	if s.Spec.TimeLimit > 0 {
		e.TimeLimit = ui.Duration(s.Spec.TimeLimit.D())
	}
	return e
}

func (a *App) listCmd() *cobra.Command {
	var pack, domain string
	var asJSON, all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List scenarios",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat := a.catalog()
			entries := []listEntry{}
			for _, s := range cat.Scenarios {
				switch {
				case pack != "" && s.Pack != pack:
				case domain != "" && s.Metadata.Domain != domain:
				case s.Metadata.Experimental && !all:
				default:
					entries = append(entries, entryOf(s))
				}
			}
			for _, p := range cat.Problems {
				ui.Warn(cmd.ErrOrStderr(), "%v", p)
			}
			if asJSON {
				return ui.JSON(cmd.OutOrStdout(), entries)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fprintf(tw, "ID\tDOMAIN\tSIZE\tTIME\n")
			for _, e := range entries {
				fprintf(tw, "%s\t%s\t%d\t%s\n", e.ID, e.Domain, e.Nodes, e.TimeLimit)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&pack, "exam", "", "only scenarios of this pack, e.g. cka")
	cmd.Flags().StringVar(&domain, "domain", "", "only scenarios of this domain")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include experimental scenarios")
	return cmd
}

func (a *App) showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show scenario metadata (no prompt, no spoilers)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.catalog().Find(args[0])
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fprintf(w, "ID:          %s\n", s.ID)
			fprintf(w, "Title:       %s\n", s.Metadata.Title)
			fprintf(w, "Domain:      %s\n", s.Metadata.Domain)
			if s.Metadata.Difficulty != "" {
				fprintf(w, "Difficulty:  %s\n", s.Metadata.Difficulty)
			}
			if len(s.Metadata.Tags) > 0 {
				fprintf(w, "Tags:        %s\n", strings.Join(s.Metadata.Tags, ", "))
			}
			if s.Spec.TimeLimit > 0 {
				fprintf(w, "Time:        %s (suggested)\n", ui.Duration(s.Spec.TimeLimit.D()))
			}
			if s.Spec.Weight > 0 {
				fprintf(w, "Weight:      %d%%\n", s.Spec.Weight)
			}
			fprintf(w, "Nodes:       %d\n", s.Spec.Cluster.CPs()+s.Spec.Cluster.Workers)
			fprintf(w, "Hints:       %d\n", len(s.Spec.Hints))
			fprintf(w, "Checks:      %d (%s to pass)\n", len(s.Spec.Checks), fmt.Sprintf("%d%%", s.Spec.PassPercent))
			return nil
		},
	}
}
