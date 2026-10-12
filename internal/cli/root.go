// Package cli is the cobra command tree. It stays thin: every command calls
// into lab, validate or scenario.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/backend/kind"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/lab"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/session"
	"github.com/JoshSLawrence/kubestronaut/scenarios"
)

// Exit codes (design section 1.1).
const (
	ExitOK        = 0
	ExitNotPassed = 1
	ExitError     = 2
	ExitNoSession = 3
)

// ExitCodeError carries a specific exit code. A nil Err means the command already
// printed everything it wanted to say.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error { return e.Err }

// App holds the global flags and the injectable dependencies.
type App struct {
	ScenariosDir string
	Home         string
	Verbose      bool
	Version      string

	// Test seams; zero values select the real implementations.
	Layers  []fs.FS
	Backend backend.Backend
	Runner  execx.Runner
	Binary  string
}

func (a *App) home() (string, error) {
	if a.Home != "" {
		return a.Home, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find your home directory (%w): pass --home or set KUBESTRONAUT_HOME", err)
	}
	return filepath.Join(h, ".kubestronaut"), nil
}

func (a *App) runner() execx.Runner {
	if a.Runner != nil {
		return a.Runner
	}
	return execx.OS{}
}

func (a *App) catalog() *scenario.Catalog {
	layers := a.Layers
	if layers == nil {
		layers = []fs.FS{scenarios.FS}
		if a.ScenariosDir != "" {
			layers = append(layers, os.DirFS(a.ScenariosDir))
		}
	}
	return scenario.LoadCatalog(layers...)
}

func (a *App) store() (*session.Store, error) {
	h, err := a.home()
	if err != nil {
		return nil, err
	}
	return &session.Store{Root: h}, nil
}

func (a *App) binary() string {
	if a.Binary != "" {
		return a.Binary
	}
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "kubestronaut"
}

func (a *App) backendFor(store *session.Store) backend.Backend {
	if a.Backend != nil {
		return a.Backend
	}
	return kind.New(store.ScratchKubeconfig())
}

// lab builds the orchestrator for this invocation.
func (a *App) lab(cmd *cobra.Command) (*lab.Lab, error) {
	store, err := a.store()
	if err != nil {
		return nil, err
	}
	l := lab.New(a.catalog(), a.backendFor(store), store, a.runner(), a.binary(), cmd.OutOrStdout())
	l.Verbose = a.Verbose
	return l, nil
}

// NewRoot builds the command tree.
func NewRoot(a *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "kubestronaut",
		Short:         "Kubernetes certification lab scenarios on kind",
		Long:          "kubestronaut creates a kind cluster with a scenario set up, shows the task, and grades your fix.\nStart with `kubestronaut list`.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	f := root.PersistentFlags()
	f.StringVar(&a.ScenariosDir, "scenarios-dir", os.Getenv("KUBESTRONAUT_SCENARIOS"), "extra scenarios directory that adds to or overrides the built-in packs (env KUBESTRONAUT_SCENARIOS)")
	f.StringVar(&a.Home, "home", os.Getenv("KUBESTRONAUT_HOME"), "state directory (env KUBESTRONAUT_HOME, default ~/.kubestronaut)")
	f.BoolVarP(&a.Verbose, "verbose", "v", false, "verbose output, including setup logs")

	root.AddCommand(
		a.listCmd(), a.showCmd(), a.startCmd(), a.statusCmd(), a.promptCmd(), a.envCmd(),
		a.shellCmd(), a.sshCmd(), a.hintCmd(), a.gradeCmd(), a.solutionCmd(), a.resetCmd(),
		a.stopCmd(), a.validateCmd(), a.doctorCmd(), a.versionCmd(),
	)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context, a *App, args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	root := NewRoot(a)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetIn(stdin)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ec *ExitCodeError
	switch {
	case errors.As(err, &ec):
		if ec.Err != nil {
			fprintf(stderr, "error: %v\n", ec.Err)
		}
		return ec.Code
	case errors.Is(err, session.ErrNoSession):
		fprintf(stderr, "error: %v\n", err)
		return ExitNoSession
	}
	fprintf(stderr, "error: %v\n", err)
	return ExitError
}

// fprintf ignores write errors: there is nothing useful to do when the
// terminal is gone.
func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// confirm asks a yes/no question on stdin unless yes is set.
func confirm(cmd *cobra.Command, yes bool, question string) error {
	if yes {
		return nil
	}
	fprintf(cmd.ErrOrStderr(), "%s [y/N] ", question)
	var ans string
	_, _ = fmt.Fscanln(cmd.InOrStdin(), &ans)
	if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
		return &ExitCodeError{Code: ExitError, Err: errors.New("cancelled: pass --yes to skip this question")}
	}
	return nil
}
