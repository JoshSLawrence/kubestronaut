package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func execxCmd(name string, args []string, env ...string) execx.Cmd {
	return execx.Cmd{Name: name, Args: args, Env: env}
}

// stdinForExec hands stdin to the node only when it is a pipe or file, so an
// interactive terminal is not swallowed by a one-shot command.
func stdinForExec(in io.Reader) io.Reader {
	f, ok := in.(*os.File)
	if !ok {
		return in
	}
	st, err := f.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	return in
}

func (a *App) sshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <node> [-- cmd...]",
		Short: "Open a shell on a node, or run a command there (controlplane, node01, ...)",
		Long: "Open a shell on a node, or run a command there.\n" +
			"Nodes are named controlplane (controlplane02, controlplane03 in an HA\n" +
			"cluster), node01, node02, and so on.\n" +
			"Set KS_CLUSTER to target a specific cluster.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := a.lab(cmd)
			if err != nil {
				return err
			}
			code, err := l.SSH(cmd.Context(), os.Getenv("KS_CLUSTER"), args[0], args[1:],
				stdinForExec(cmd.InOrStdin()), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if code != 0 {
				return &ExitCodeError{Code: code}
			}
			return nil
		},
	}
}
