// Package kubeconfig builds the user-facing kubeconfig from the admin
// kubeconfigs kind writes, without touching ~/.kube/config.
package kubeconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// Entry is one cluster to include.
type Entry struct {
	// Admin is the kubeconfig kind wrote for the cluster.
	Admin string
	// Cluster is the kind cluster name; kind names its context kind-<name>.
	Cluster string
	// Context is the name the user sees, e.g. k8s-kubelet.
	Context string
}

// KindContext is the context name kind gives a cluster.
func KindContext(cluster string) string { return "kind-" + cluster }

// Generate writes out as the merge of entries with contexts renamed, and
// selects current. With no entries the file is removed. Merging goes through
// `kubectl config view --flatten` so no client-go dependency is needed.
func Generate(ctx context.Context, r execx.Runner, entries []Entry, current, out string) error {
	if len(entries) == 0 {
		if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(dir, "kubeconfig-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	var files []string
	for i, e := range entries {
		raw, err := os.ReadFile(e.Admin)
		if err != nil {
			return fmt.Errorf("read admin kubeconfig for %s: %w", e.Cluster, err)
		}
		name := fmt.Sprintf("%d.kubeconfig", i)
		if err := root.WriteFile(name, raw, 0o600); err != nil {
			return err
		}
		cp := filepath.Join(tmpDir, name)
		if err := kubectl(ctx, r, nil, "config", "--kubeconfig", cp, "rename-context", KindContext(e.Cluster), e.Context); err != nil {
			return err
		}
		files = append(files, cp)
	}

	res, err := r.Run(ctx, execx.Cmd{
		Name: "kubectl", Args: []string{"config", "view", "--flatten", "--raw"},
		Env: []string{"KUBECONFIG=" + joinPaths(files)},
	})
	if err != nil {
		return fmt.Errorf("kubectl config view: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("kubectl config view failed: %s", res.Stderr)
	}
	if err := os.WriteFile(out, res.Stdout, 0o600); err != nil {
		return err
	}
	return kubectl(ctx, r, nil, "config", "--kubeconfig", out, "use-context", current)
}

func joinPaths(p []string) string {
	s := ""
	for i, v := range p {
		if i > 0 {
			s += string(os.PathListSeparator)
		}
		s += v
	}
	return s
}

func kubectl(ctx context.Context, r execx.Runner, env []string, args ...string) error {
	res, err := r.Run(ctx, execx.Cmd{Name: "kubectl", Args: args, Env: env})
	if err != nil {
		return fmt.Errorf("kubectl %s: %w", args[0], err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("kubectl %s failed: %s", strings.Join(args[:min(len(args), 4)], " "), strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}
