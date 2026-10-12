// Package setup runs the ordered setup steps of a scenario.
package setup

import (
	"bytes"
	"context"
	_ "embed" // lib.sh is embedded
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

//go:embed lib.sh
var libSh []byte

// TotalTimeout bounds all setup steps together.
const TotalTimeout = 15 * time.Minute

// pollInterval is how often a wait step re-checks that a resource exists.
var pollInterval = 2 * time.Second

// WriteLib writes the helper library into dir and returns its path.
func WriteLib(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "lib.sh")
	if err := os.WriteFile(p, libSh, 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// EnvParams are the values scripts receive.
type EnvParams struct {
	AdminKubeconfig string
	Binary          string // path of the kubestronaut binary, KS_BIN
	CacheDir        string // kubectl discovery cache, kept out of ~/.kube
	Home            string // state dir, so nested kubestronaut calls agree
	Cluster         string
	Context         string
	ScenarioDir     string
	Lib             string
	Nodes           []backend.Node
}

// ScriptEnv builds the environment for setup, solution and check scripts.
func ScriptEnv(p EnvParams) []string {
	env := []string{
		"KUBECONFIG=" + p.AdminKubeconfig,
		"KS_BIN=" + p.Binary,
		"KUBESTRONAUT_HOME=" + p.Home,
		"KS_CLUSTER=" + p.Cluster,
		"KS_CONTEXT=" + p.Context,
		"KS_SCENARIO_DIR=" + p.ScenarioDir,
		"KS_LIB=" + p.Lib,
	}
	if p.CacheDir != "" {
		env = append(env, "KUBECACHEDIR="+p.CacheDir)
	}
	for _, n := range p.Nodes {
		env = append(env, "KS_NODE_"+strings.ToUpper(n.Alias)+"="+n.Name)
	}
	return env
}

// Runner executes setup steps.
type Runner struct {
	Kube kube.Client
	Exec execx.Runner
	// Log receives script output and step headers (setup.log); may be nil.
	Log io.Writer
	// Env is passed to scripts.
	Env []string
	// Dir is the materialized scenario directory.
	Dir string
}

func (r *Runner) logf(format string, a ...any) {
	if r.Log != nil {
		_, _ = fmt.Fprintf(r.Log, format+"\n", a...)
	}
}

// Run executes the scenario's setup steps in order.
func (r *Runner) Run(ctx context.Context, s *scenario.Scenario) error {
	ctx, cancel := context.WithTimeout(ctx, TotalTimeout)
	defer cancel()
	steps := s.Spec.Setup
	for i, st := range steps {
		var err error
		switch {
		case st.Script != "":
			r.logf("== step %d/%d: script %s", i+1, len(steps), st.Script)
			err = r.script(ctx, st)
		case st.Manifest != "":
			r.logf("== step %d/%d: manifest %s", i+1, len(steps), st.Manifest)
			err = r.manifest(ctx, st)
		case st.Wait != nil:
			r.logf("== step %d/%d: wait %s for %s", i+1, len(steps), st.Wait.Ref.Kind, st.Wait.For)
			err = r.wait(ctx, st)
		}
		if err != nil {
			return fmt.Errorf("setup step %d of %d: %w", i+1, len(steps), err)
		}
	}
	return nil
}

func tail(b []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (r *Runner) script(ctx context.Context, st scenario.SetupStep) error {
	sctx, cancel := context.WithTimeout(ctx, st.Timeout.D())
	defer cancel()
	res, err := r.Exec.Run(sctx, execx.Cmd{Name: "bash", Args: []string{st.Script}, Env: r.Env, Dir: r.Dir})
	r.logf("%s%s", res.Stdout, res.Stderr)
	if err != nil {
		if errors.Is(sctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("script %s timed out after %s", st.Script, st.Timeout.D())
		}
		return fmt.Errorf("script %s: %w", st.Script, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("script %s exited %d: %s", st.Script, res.ExitCode, tail(append(res.Stdout, res.Stderr...), 5))
	}
	return nil
}

// manifestFiles returns the files a manifest step covers, sorted.
func (r *Runner) manifestFiles(path string) ([]string, error) {
	full := filepath.Join(r.Dir, path)
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{full}, nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".yaml", ".yml", ".json":
			if !e.IsDir() {
				out = append(out, filepath.Join(full, e.Name()))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *Runner) manifest(ctx context.Context, st scenario.SetupStep) error {
	files, err := r.manifestFiles(st.Manifest)
	if err != nil {
		return fmt.Errorf("manifest %s: %w", st.Manifest, err)
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			return err
		}
		if err := r.Kube.Apply(ctx, bytes.TrimSpace(b)); err != nil {
			return fmt.Errorf("manifest %s: %w", st.Manifest, err)
		}
	}
	return nil
}

func (r *Runner) wait(ctx context.Context, st scenario.SetupStep) error {
	w := st.Wait
	timeout := waitTimeout(st)
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if w.For != "delete" {
		// kubectl wait fails at once for objects that do not exist yet, which
		// is normal right after an apply, so wait for them to appear first.
		for {
			objs, err := r.Kube.Get(wctx, w.Ref)
			if err == nil && len(objs) > 0 {
				break
			}
			select {
			case <-wctx.Done():
				return fmt.Errorf("wait: %s did not appear within %s", w.Ref.Kind, timeout)
			case <-time.After(pollInterval):
			}
		}
	}
	remaining := timeout
	if dl, ok := wctx.Deadline(); ok {
		remaining = max(time.Until(dl), time.Second)
	}
	if err := r.Kube.Wait(wctx, w.Ref, w.For, remaining); err != nil {
		return fmt.Errorf("wait for %s on %s: %w", w.For, w.Ref.Kind, err)
	}
	return nil
}

// waitTimeout resolves wait.timeout, then the step timeout, then the default.
func waitTimeout(st scenario.SetupStep) time.Duration {
	switch {
	case st.Wait.Timeout > 0:
		return st.Wait.Timeout.D()
	case st.Timeout > 0:
		return st.Timeout.D()
	}
	return scenario.DefaultWaitTimeout
}
