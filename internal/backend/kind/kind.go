// Package kind implements backend.Backend by running the kind and docker
// command line tools.
package kind

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// Prefix marks every cluster kubestronaut owns. Delete and List refuse
// anything else so a user's own kind clusters are never touched.
const Prefix = "ks-"

// WaitForControlPlane is passed to `kind create cluster --wait`.
const WaitForControlPlane = "3m"

// WaitNone is used instead when the cluster has no CNI: its nodes cannot
// become Ready, so waiting would idle for the whole WaitForControlPlane.
const WaitNone = "0s"

// Kind runs clusters through the kind CLI.
type Kind struct {
	Runner execx.Runner
	// Scratch is a kubeconfig path used for commands that would otherwise
	// edit the user's ~/.kube/config (kind delete removes contexts from it).
	Scratch string
}

// New returns a Kind that uses the real kind and docker binaries.
func New(scratchKubeconfig string) *Kind {
	return &Kind{Runner: execx.OS{}, Scratch: scratchKubeconfig}
}

// Name implements backend.Backend.
func (*Kind) Name() string { return "kind" }

func (k *Kind) run(ctx context.Context, name string, args ...string) (execx.Result, error) {
	res, err := k.Runner.Run(ctx, execx.Cmd{Name: name, Args: args})
	if err != nil {
		return res, fmt.Errorf("%s %s: %w (is %s installed and on PATH?)", name, args[0], err, name)
	}
	if res.ExitCode != 0 {
		return res, fmt.Errorf("%s %s failed (exit %d): %s", name, strings.Join(args[:min(len(args), 2)], " "),
			res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return res, nil
}

func ownedName(name string) error {
	if !strings.HasPrefix(name, Prefix) {
		return fmt.Errorf("refusing to touch cluster %q: kubestronaut only manages clusters named %s*", name, Prefix)
	}
	return nil
}

// Create implements backend.Backend.
func (k *Kind) Create(ctx context.Context, s backend.ClusterSpec) error {
	if err := ownedName(s.Name); err != nil {
		return err
	}
	image := s.NodeImage
	var err error
	if image == "" {
		// Normally a no-op because Prepare already built it; repeated here so
		// Create never hands kind a tag that does not exist locally.
		if image, err = k.buildNodeImage(ctx, s.KubernetesVersion); err != nil {
			return err
		}
	}
	dir := filepath.Dir(s.Kubeconfig)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	cfg := filepath.Join(dir, "kind-"+s.Name+".yaml")
	if err := os.WriteFile(cfg, buildConfig(s, image, hostMounts()), 0o600); err != nil {
		return fmt.Errorf("write kind config: %w", err)
	}
	wait := WaitForControlPlane
	if s.DisableDefaultCNI {
		wait = WaitNone
	}
	_, err = k.run(ctx, "kind", "create", "cluster", "--name", s.Name, "--config", cfg,
		"--kubeconfig", s.Kubeconfig, "--wait", wait)
	return err
}

// Delete implements backend.Backend. Deleting a missing cluster succeeds.
func (k *Kind) Delete(ctx context.Context, name string) error {
	if err := ownedName(name); err != nil {
		return err
	}
	args := []string{"delete", "cluster", "--name", name}
	if k.Scratch != "" {
		args = append(args, "--kubeconfig", k.Scratch)
	}
	_, err := k.run(ctx, "kind", args...)
	return err
}

// List implements backend.Backend.
func (k *Kind) List(ctx context.Context) ([]string, error) {
	res, err := k.run(ctx, "kind", "get", "clusters")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(res.Stdout), "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, Prefix) {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out, nil
}

var (
	workerRE       = regexp.MustCompile(`-worker(\d*)$`)
	controlPlaneRE = regexp.MustCompile(`-control-plane(\d*)$`)
)

// loadBalancerSuffix ends the name of the container kind adds in front of the
// API servers of an HA cluster. It is not a Kubernetes node: it has no
// kubelet, no containerd and no alias.
const loadBalancerSuffix = "-external-load-balancer"

// LoadBalancerImage is the image kind v0.33.0 runs as that load balancer
// (loadbalancer.Image in its source). Keep it in step with kind in mise.toml.
// Prepare pulls it first, so the pull does not count against the create
// timeout.
const LoadBalancerImage = "docker.io/envoyproxy/envoy:v1.36.2"

// Nodes implements backend.Backend. Aliases follow the order described in
// backend.NodeName: controlplane, controlplane02, ... by control-plane index,
// then node01, node02, ... by worker index. The load balancer of an HA cluster
// is skipped.
func (k *Kind) Nodes(ctx context.Context, cluster string) ([]backend.Node, error) {
	res, err := k.run(ctx, "kind", "get", "nodes", "--name", cluster)
	if err != nil {
		return nil, err
	}
	type worker struct {
		name string
		idx  int
	}
	type controlPlane struct {
		name string
		idx  int
	}
	var cps []controlPlane
	var ws []worker
	for _, l := range strings.Split(string(res.Stdout), "\n") {
		l = strings.TrimSpace(l)
		switch {
		case l == "":
		case strings.HasSuffix(l, loadBalancerSuffix):
		case controlPlaneRE.MatchString(l):
			idx := 1 // kind names the first control plane "-control-plane" and then -control-plane2, -control-plane3
			if m := controlPlaneRE.FindStringSubmatch(l); m[1] != "" {
				idx, _ = strconv.Atoi(m[1])
			}
			cps = append(cps, controlPlane{l, idx})
		case workerRE.MatchString(l):
			idx := 1 // kind names the first worker "-worker" and then -worker2, -worker3
			if m := workerRE.FindStringSubmatch(l); m[1] != "" {
				idx, _ = strconv.Atoi(m[1])
			}
			ws = append(ws, worker{l, idx})
		default:
			return nil, fmt.Errorf("unexpected node %q in cluster %s", l, cluster)
		}
	}
	if len(cps) == 0 {
		return nil, fmt.Errorf("cluster %s has no control-plane node: it may not exist (run `kubestronaut doctor`)", cluster)
	}
	sort.Slice(cps, func(i, j int) bool { return cps[i].idx < cps[j].idx })
	sort.Slice(ws, func(i, j int) bool { return ws[i].idx < ws[j].idx })
	var nodes []backend.Node
	for i, c := range cps {
		nodes = append(nodes, backend.Node{Name: c.name, Alias: backend.ControlPlaneAliasN(i + 1), Role: backend.RoleControlPlane})
	}
	for i, w := range ws {
		nodes = append(nodes, backend.Node{Name: w.name, Alias: backend.WorkerAlias(i + 1), Role: backend.RoleWorker})
	}
	return nodes, nil
}

// NodeExec implements backend.Backend.
func (k *Kind) NodeExec(ctx context.Context, cluster, alias string, stdin io.Reader, argv ...string) (execx.Result, error) {
	if err := ownedName(cluster); err != nil {
		return execx.Result{}, err
	}
	node, err := backend.NodeName(cluster, alias)
	if err != nil {
		return execx.Result{}, err
	}
	args := []string{"exec"}
	if stdin != nil {
		args = append(args, "-i")
	}
	args = append(args, node)
	args = append(args, argv...)
	res, err := k.Runner.Run(ctx, execx.Cmd{Name: "docker", Args: args, Stdin: stdin})
	if err != nil {
		return res, fmt.Errorf("docker exec on %s: %w (is docker running?)", alias, err)
	}
	return res, nil
}

// NodeShell implements backend.Backend.
func (k *Kind) NodeShell(ctx context.Context, cluster, alias string) error {
	if err := ownedName(cluster); err != nil {
		return err
	}
	node, err := backend.NodeName(cluster, alias)
	if err != nil {
		return err
	}
	return k.Runner.Interactive(ctx, execx.Cmd{Name: "docker", Args: []string{"exec", "-it", node, "bash", "-l"}})
}

// LoadImages implements backend.Backend. Every image is attempted; failures
// are joined so the caller can warn and let the nodes pull instead.
func (k *Kind) LoadImages(ctx context.Context, cluster string, images []string) error {
	var errs []error
	for _, img := range images {
		res, err := k.Runner.Run(ctx, execx.Cmd{Name: "docker", Args: []string{"image", "inspect", img}})
		if err != nil {
			return fmt.Errorf("docker image inspect: %w", err)
		}
		if res.ExitCode != 0 {
			if _, err := k.run(ctx, "docker", "pull", img); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		_, err = k.run(ctx, "kind", "load", "docker-image", img, "--name", cluster)
		if err == nil {
			continue
		}
		// With Docker's containerd image store, kind load fails on multi-arch
		// images because the index references platforms that were never
		// pulled. A single-platform save sidesteps that.
		if ferr := k.loadViaSave(ctx, cluster, img); ferr != nil {
			errs = append(errs, err, ferr)
		}
	}
	return errors.Join(errs...)
}

// loadViaSave streams a single-platform `docker save` into every node.
func (k *Kind) loadViaSave(ctx context.Context, cluster, img string) error {
	platform := "linux/" + runtime.GOARCH
	res, err := k.run(ctx, "docker", "save", "--platform", platform, img)
	if err != nil {
		// Docker's containerd image store can hold an entry for an image
		// whose layers it cannot export (seen with registry.k8s.io/etcd
		// 3.7.0-0 while 3.6.8-0 exported fine). A fresh pull of the platform
		// is a cheap retry that may refresh the entry; it did not for that
		// etcd image on Docker 29.8.2, so callers must still cope with a
		// failed load.
		if _, perr := k.run(ctx, "docker", "pull", "--platform", platform, img); perr != nil {
			return errors.Join(err, perr)
		}
		if res, err = k.run(ctx, "docker", "save", "--platform", platform, img); err != nil {
			return err
		}
	}
	nodes, err := k.Nodes(ctx, cluster)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		r, err := k.Runner.Run(ctx, execx.Cmd{
			Name: "docker",
			Args: []string{"exec", "--privileged", "-i", n.Name, "ctr", "--namespace=k8s.io", "images", "import",
				"--digests", "--snapshotter=overlayfs", "-"},
			Stdin: bytes.NewReader(res.Stdout),
		})
		if err != nil {
			return fmt.Errorf("import %s into %s: %w", img, n.Alias, err)
		}
		if r.ExitCode != 0 {
			return fmt.Errorf("import %s into %s failed (exit %d): %s", img, n.Alias, r.ExitCode, strings.TrimSpace(string(r.Stderr)))
		}
	}
	return nil
}
