// Package fake is an in-memory backend.Backend for tests.
package fake

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// ExecCall records one NodeExec.
type ExecCall struct {
	Cluster, Alias string
	Stdin          string
	Argv           []string
}

// Fake implements backend.Backend.
type Fake struct {
	mu sync.Mutex

	// Clusters maps a cluster name to its nodes.
	Clusters map[string][]backend.Node
	// ExecFunc answers NodeExec; nil means exit 0 with no output.
	ExecFunc func(cluster, alias string, argv []string) (execx.Result, error)
	// PrepareErr, CreateErr, DeleteErr, ShellErr and LoadErr make those
	// calls fail.
	PrepareErr, CreateErr, DeleteErr, ShellErr, LoadErr error
	// PrepareNotify, when set, is passed to notify by Prepare.
	PrepareNotify string
	// ImageBuilt and ImageErr answer NodeImageBuilt; Stale answers
	// StaleNodeImages.
	ImageBuilt bool
	ImageErr   error
	Stale      []string

	Prepared []backend.ClusterSpec
	Created  []backend.ClusterSpec
	Deleted  []string
	Execs    []ExecCall
	Shells   []string
	Loaded   map[string][]string
}

// New returns an empty fake.
func New() *Fake {
	return &Fake{Clusters: map[string][]backend.Node{}, Loaded: map[string][]string{}}
}

// Name implements backend.Backend.
func (f *Fake) Name() string { return "fake" }

// Prepare implements backend.Backend.
func (f *Fake) Prepare(_ context.Context, s backend.ClusterSpec, notify func(string)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.PrepareErr != nil {
		return f.PrepareErr
	}
	f.Prepared = append(f.Prepared, s)
	if f.PrepareNotify != "" && notify != nil {
		notify(f.PrepareNotify)
	}
	return nil
}

// NodeImageBuilt mirrors the kind backend's method so doctor can be tested.
func (f *Fake) NodeImageBuilt(_ context.Context, version string) (string, bool, error) {
	return "fake/node:" + version, f.ImageBuilt, f.ImageErr
}

// StaleNodeImages mirrors the kind backend's method so doctor can be tested.
func (f *Fake) StaleNodeImages(context.Context) ([]string, error) { return f.Stale, nil }

// Create implements backend.Backend. It builds the same nodes kind would.
func (f *Fake) Create(_ context.Context, s backend.ClusterSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateErr != nil {
		return f.CreateErr
	}
	f.Created = append(f.Created, s)
	f.Clusters[s.Name] = NodesFor(s.Name, max(s.ControlPlanes, 1), s.Workers)
	if s.Kubeconfig != "" {
		// Like kind, leave an admin kubeconfig behind.
		if err := os.MkdirAll(filepath.Dir(s.Kubeconfig), 0o700); err != nil {
			return err
		}
		return os.WriteFile(s.Kubeconfig, []byte("admin of "+s.Name), 0o600)
	}
	return nil
}

// Nodes builds the node list for a cluster with one control plane and the
// given worker count.
func Nodes(cluster string, workers int) []backend.Node { return NodesFor(cluster, 1, workers) }

// NodesFor builds the node list for a cluster with the given numbers of
// control planes and workers.
func NodesFor(cluster string, controlPlanes, workers int) []backend.Node {
	var nodes []backend.Node
	for i := 1; i <= controlPlanes; i++ {
		a := backend.ControlPlaneAliasN(i)
		n, _ := backend.NodeName(cluster, a)
		nodes = append(nodes, backend.Node{Name: n, Alias: a, Role: backend.RoleControlPlane})
	}
	for i := 1; i <= workers; i++ {
		a := backend.WorkerAlias(i)
		n, _ := backend.NodeName(cluster, a)
		nodes = append(nodes, backend.Node{Name: n, Alias: a, Role: backend.RoleWorker})
	}
	return nodes
}

// Delete implements backend.Backend.
func (f *Fake) Delete(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	f.Deleted = append(f.Deleted, name)
	delete(f.Clusters, name)
	return nil
}

// List implements backend.Backend.
func (f *Fake) List(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for n := range f.Clusters {
		if strings.HasPrefix(n, "ks-") {
			out = append(out, n)
		}
	}
	return out, nil
}

// Nodes implements backend.Backend.
func (f *Fake) Nodes(_ context.Context, cluster string) ([]backend.Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("fake: no cluster %q", cluster)
	}
	return n, nil
}

// NodeExec implements backend.Backend.
func (f *Fake) NodeExec(_ context.Context, cluster, alias string, stdin io.Reader, argv ...string) (execx.Result, error) {
	call := ExecCall{Cluster: cluster, Alias: alias, Argv: argv}
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		call.Stdin = string(b)
	}
	f.mu.Lock()
	f.Execs = append(f.Execs, call)
	fn := f.ExecFunc
	f.mu.Unlock()
	if fn == nil {
		return execx.Result{}, nil
	}
	return fn(cluster, alias, argv)
}

// NodeShell implements backend.Backend.
func (f *Fake) NodeShell(_ context.Context, cluster, alias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Shells = append(f.Shells, cluster+"/"+alias)
	return f.ShellErr
}

// LoadImages implements backend.Backend.
func (f *Fake) LoadImages(_ context.Context, cluster string, images []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Loaded[cluster] = append(f.Loaded[cluster], images...)
	return f.LoadErr
}
