// Package backend abstracts the local Kubernetes cluster provider.
package backend

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// NodeRole is a node's role in the cluster.
type NodeRole string

// Node roles.
const (
	RoleControlPlane NodeRole = "control-plane"
	RoleWorker       NodeRole = "worker"
)

// ClusterSpec describes a cluster to create.
type ClusterSpec struct {
	Name              string // ks-<slug>, at most 40 characters
	KubernetesVersion string // e.g. v1.36.4, resolved to a pinned image
	NodeImage         string // optional raw override
	ControlPlanes     int    // 1 only for now
	Workers           int
	Kubeconfig        string // path the admin kubeconfig is written to
	KubeadmPatches    []string
	// DisableDefaultCNI creates the cluster without a CNI plugin. The nodes
	// cannot become Ready, so Create must not wait for them.
	DisableDefaultCNI bool
}

// Node is one cluster node.
type Node struct {
	Name  string   // real node name, e.g. ks-x-worker
	Alias string   // controlplane, node01, node02, ...
	Role  NodeRole // control-plane or worker
}

// Backend creates and operates clusters.
type Backend interface {
	Name() string
	// Prepare fetches or builds whatever Create needs and can be slow the
	// first time (for kind, building the node image). It calls notify before
	// any slow work so the caller can tell the user why it is waiting.
	Prepare(ctx context.Context, s ClusterSpec, notify func(msg string)) error
	Create(ctx context.Context, s ClusterSpec) error
	Delete(ctx context.Context, name string) error
	// List returns only clusters whose names start with the ks- prefix.
	List(ctx context.Context) ([]string, error)
	Nodes(ctx context.Context, cluster string) ([]Node, error)
	NodeExec(ctx context.Context, cluster, alias string, stdin io.Reader, argv ...string) (execx.Result, error)
	NodeShell(ctx context.Context, cluster, alias string) error
	LoadImages(ctx context.Context, cluster string, images []string) error
}

// DefaultKubernetesVersion is used when neither the scenario nor the user
// names a version.
const DefaultKubernetesVersion = "v1.36.4"

// ControlPlaneAlias is the alias of the single control-plane node.
const ControlPlaneAlias = "controlplane"

var aliasRE = regexp.MustCompile(`^node(\d{2,})$`)

// WorkerIndex parses node01, node02, ... into 1, 2, ...
func WorkerIndex(alias string) (int, bool) {
	m := aliasRE.FindStringSubmatch(alias)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// WorkerAlias formats the alias of the n-th (1-based) worker.
func WorkerAlias(n int) string { return fmt.Sprintf("node%02d", n) }

// NodeName is the real node name for an alias. It follows kind's naming
// (<cluster>-control-plane, <cluster>-worker, <cluster>-worker2, ...) and is
// the contract every backend must honour so scenario templates can resolve
// names before a cluster exists.
func NodeName(cluster, alias string) (string, error) {
	if alias == ControlPlaneAlias {
		return cluster + "-control-plane", nil
	}
	n, ok := WorkerIndex(alias)
	if !ok {
		return "", fmt.Errorf("unknown node alias %q: use %q, node01, node02 and so on", alias, ControlPlaneAlias)
	}
	if n == 1 {
		return cluster + "-worker", nil
	}
	return fmt.Sprintf("%s-worker%d", cluster, n), nil
}
