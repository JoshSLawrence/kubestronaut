package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
)

func init() {
	Register("nodeExec", func(raw json.RawMessage) (Check, error) {
		var c nodeExec
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if c.Node == "" {
			return nil, errors.New("node is required")
		}
		var err error
		if c.argv, c.re, err = c.compile(false); err != nil {
			return nil, err
		}
		return &c, nil
	})
	Register("podExec", func(raw json.RawMessage) (Check, error) {
		var c podExec
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if err := validRef(c.Ref); err != nil {
			return nil, err
		}
		if c.Ref.Name == "" && c.Ref.Selector == "" {
			return nil, errors.New("ref.name or ref.selector is required")
		}
		var err error
		if c.argv, c.re, err = c.compile(true); err != nil {
			return nil, err
		}
		return &c, nil
	})
}

// Expect describes a command and what its result must look like. The command
// is either an argv list or a shell string.
type Expect struct {
	Command        []string `json:"command"`
	Shell          string   `json:"shell"`
	ExitCode       *int     `json:"exitCode"`
	StdoutContains string   `json:"stdoutContains"`
	StdoutMatches  string   `json:"stdoutMatches"`
}

// podShellPrelude makes a pod's shell stop at the first failing command, like
// the node's bash -e. pipefail is enabled where the shell has it (bash, busybox
// ash, recent dash) and skipped where it does not.
const podShellPrelude = `set -e; if (set -o pipefail) 2>/dev/null; then set -o pipefail; fi; eval "$1"`

// shellArgv wraps a shell string so that a failing command anywhere in it
// fails the check, not only the last one. On nodes that is bash with errexit
// and pipefail. Pods may only have sh, so the script is passed as an argument
// to a small wrapper; -u is left off in both so an unset variable is not an
// error.
func shellArgv(script string, pod bool) []string {
	if pod {
		return []string{"sh", "-c", podShellPrelude, "sh", script}
	}
	return []string{"bash", "-eo", "pipefail", "-c", script}
}

func (e Expect) compile(pod bool) ([]string, *regexp.Regexp, error) {
	var argv []string
	switch {
	case len(e.Command) > 0 && e.Shell != "":
		return nil, nil, errors.New("command and shell are mutually exclusive")
	case len(e.Command) > 0:
		argv = e.Command
	case e.Shell != "":
		argv = shellArgv(e.Shell, pod)
	default:
		return nil, nil, errors.New("command or shell is required")
	}
	var re *regexp.Regexp
	if e.StdoutMatches != "" {
		var err error
		if re, err = regexp.Compile(e.StdoutMatches); err != nil {
			return nil, nil, fmt.Errorf("stdoutMatches: %w", err)
		}
	}
	return argv, re, nil
}

func (e Expect) judge(res execx.Result, re *regexp.Regexp) Result {
	want := 0
	if e.ExitCode != nil {
		want = *e.ExitCode
	}
	if res.ExitCode != want {
		return fail("command exited %d, expected %d", res.ExitCode, want)
	}
	out := string(res.Stdout)
	if e.StdoutContains != "" && !strings.Contains(out, e.StdoutContains) {
		return fail("command output does not contain the expected text")
	}
	if re != nil && !re.MatchString(out) {
		return fail("command output does not match the expected pattern")
	}
	return pass()
}

type nodeExec struct {
	Node string `json:"node"`
	Expect

	argv []string
	re   *regexp.Regexp
}

func (c *nodeExec) Run(ctx context.Context, env *Env) (Result, error) {
	res, err := env.Backend.NodeExec(ctx, env.Cluster, c.Node, nil, c.argv...)
	if err != nil {
		return Result{}, fmt.Errorf("exec on %s: %w", c.Node, err)
	}
	return c.judge(res, c.re), nil
}

type podExec struct {
	Ref       kube.Ref `json:"ref"`
	Container string   `json:"container"`
	Expect

	argv []string
	re   *regexp.Regexp
}

func (c *podExec) Run(ctx context.Context, env *Env) (Result, error) {
	ref := c.Ref
	if ref.Name == "" {
		objs, err := env.Kube.Get(ctx, ref)
		if err != nil {
			return Result{}, err
		}
		if len(objs) == 0 {
			return fail("%s not found", describe(ref)), nil
		}
		ref.Name, _ = field(objs[0], "metadata", "name").(string)
	}
	res, err := env.Kube.Exec(ctx, ref.Namespace, ref.Name, c.Container, c.argv)
	if err != nil {
		return Result{}, err
	}
	if res.ExitCode != 0 && isKubectlFailure(res) {
		return fail("could not run the command in %s", describe(ref)), nil
	}
	return c.judge(res, c.re), nil
}

// isKubectlFailure tells a kubectl-level failure (pod missing, container not
// running) from the exit code of the command itself.
func isKubectlFailure(res execx.Result) bool {
	e := string(res.Stderr)
	return strings.Contains(e, "Error from server") || strings.Contains(e, "error: unable to") ||
		strings.Contains(e, "error: cannot exec") || strings.Contains(e, "container not found")
}
