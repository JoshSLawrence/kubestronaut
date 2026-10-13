package kind

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func newKind() (*Kind, *execx.Fake) {
	f := &execx.Fake{}
	return &Kind{Runner: f, Scratch: "/scratch/kc"}, f
}

func TestCreateCommand(t *testing.T) {
	k, f := newKind()
	dir := t.TempDir()
	kc := filepath.Join(dir, "sess", "admin.kubeconfig")
	err := k.Create(context.Background(), backend.ClusterSpec{Name: "ks-cka-x", Workers: 1, Kubeconfig: kc})
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sess", "kind-ks-cka-x.yaml")
	want := "kind create cluster --name ks-cka-x --config " + cfg + " --kubeconfig " + kc + " --wait 3m"
	lines := f.Lines()
	if got := lines[len(lines)-1]; got != want {
		t.Fatalf("command:\n got %s\nwant %s", got, want)
	}
	b, err := os.ReadFile(filepath.Clean(cfg))
	if err != nil || !strings.Contains(string(b), "role: worker") {
		t.Fatalf("config file: %q %v", b, err)
	}
	tag, _ := NodeImageTag("")
	if !strings.Contains(string(b), "image: "+tag) {
		t.Fatalf("config must use the built node image %s:\n%s", tag, b)
	}
}

func TestCreateWithoutCNIDoesNotWait(t *testing.T) {
	k, f := newKind()
	kc := filepath.Join(t.TempDir(), "sess", "admin.kubeconfig")
	err := k.Create(context.Background(), backend.ClusterSpec{Name: "ks-cka-x", Kubeconfig: kc, DisableDefaultCNI: true})
	if err != nil {
		t.Fatal(err)
	}
	lines := f.Lines()
	if got := lines[len(lines)-1]; !strings.HasSuffix(got, " --wait 0s") {
		t.Fatalf("a cluster without a CNI cannot become Ready, so kind must not wait: %s", got)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(kc), "kind-ks-cka-x.yaml"))
	if err != nil || !strings.Contains(string(b), "disableDefaultCNI: true") {
		t.Fatalf("config file: %q %v", b, err)
	}
}

func TestCreateRejectsForeignAndBadVersion(t *testing.T) {
	k, f := newKind()
	if err := k.Create(context.Background(), backend.ClusterSpec{Name: "mine"}); err == nil {
		t.Fatal("must refuse a non ks- name")
	}
	if err := k.Create(context.Background(), backend.ClusterSpec{Name: "ks-x", KubernetesVersion: "v0", Kubeconfig: "/tmp/x/kc"}); err == nil {
		t.Fatal("must reject unknown version")
	}
	if len(f.Calls) != 0 {
		t.Fatal("nothing may run")
	}
}

func TestCreateFailureIsActionable(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"kind", "create"}, execx.Result{ExitCode: 1, Stderr: []byte("docker not running\n")}, nil)
	err := k.Create(context.Background(), backend.ClusterSpec{Name: "ks-x", Kubeconfig: filepath.Join(t.TempDir(), "kc")})
	if err == nil || !strings.Contains(err.Error(), "docker not running") {
		t.Fatalf("err: %v", err)
	}
	f.Respond([]string{"kind", "create"}, execx.Result{}, errors.New("executable file not found"))
	err = k.Create(context.Background(), backend.ClusterSpec{Name: "ks-x", Kubeconfig: filepath.Join(t.TempDir(), "kc")})
	if err == nil || !strings.Contains(err.Error(), "installed and on PATH") {
		t.Fatalf("err: %v", err)
	}
}

func TestDeleteAndList(t *testing.T) {
	k, f := newKind()
	ctx := context.Background()
	if err := k.Delete(ctx, "ks-a"); err != nil {
		t.Fatal(err)
	}
	if got := f.Lines()[0]; got != "kind delete cluster --name ks-a --kubeconfig /scratch/kc" {
		t.Fatalf("delete: %s", got)
	}
	if err := k.Delete(ctx, "prod"); err == nil {
		t.Fatal("must refuse foreign cluster")
	}
	k.Scratch = ""
	if err := k.Delete(ctx, "ks-b"); err != nil || strings.Contains(f.Lines()[1], "--kubeconfig") {
		t.Fatalf("no scratch: %v %v", err, f.Lines())
	}

	f.Respond([]string{"kind", "get", "clusters"}, execx.Result{Stdout: []byte("dev\nks-z\nks-a\n")}, nil)
	got, err := k.List(ctx)
	if err != nil || strings.Join(got, ",") != "ks-a,ks-z" {
		t.Fatalf("list: %v %v", got, err)
	}
	f.Respond([]string{"kind", "get", "clusters"}, execx.Result{ExitCode: 1, Stderr: []byte("boom")}, nil)
	if _, err := k.List(ctx); err == nil {
		t.Fatal("want error")
	}
}

func TestNodesAliases(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte(
		"ks-x-worker2\nks-x-control-plane\nks-x-worker10\nks-x-worker\nks-x-worker3\nks-x-worker4\nks-x-worker5\nks-x-worker6\nks-x-worker7\nks-x-worker8\nks-x-worker9\n")}, nil)
	nodes, err := k.Nodes(context.Background(), "ks-x")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, n.Alias+"="+n.Name)
	}
	// worker10 must sort after worker9, not after worker1.
	if len(got) != 11 || got[0] != "controlplane=ks-x-control-plane" || got[1] != "node01=ks-x-worker" ||
		got[2] != "node02=ks-x-worker2" || got[10] != "node10=ks-x-worker10" {
		t.Fatalf("nodes: %v", got)
	}
	if nodes[0].Role != backend.RoleControlPlane || nodes[1].Role != backend.RoleWorker {
		t.Fatalf("roles: %+v", nodes)
	}
	// The alias contract must agree with backend.NodeName.
	for _, n := range nodes {
		if name, _ := backend.NodeName("ks-x", n.Alias); name != n.Name {
			t.Errorf("%s: NodeName=%s listed=%s", n.Alias, name, n.Name)
		}
	}
}

func TestNodesHAClusterSkipsTheLoadBalancer(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte(
		"ks-x-control-plane3\nks-x-external-load-balancer\nks-x-worker\nks-x-control-plane2\nks-x-control-plane\n")}, nil)
	nodes, err := k.Nodes(context.Background(), "ks-x")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, n.Alias+"="+n.Name+":"+string(n.Role))
	}
	want := []string{
		"controlplane=ks-x-control-plane:control-plane",
		"controlplane02=ks-x-control-plane2:control-plane",
		"controlplane03=ks-x-control-plane3:control-plane",
		"node01=ks-x-worker:worker",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("nodes:\n got %v\nwant %v", got, want)
	}
	for _, n := range nodes {
		if name, _ := backend.NodeName("ks-x", n.Alias); name != n.Name {
			t.Errorf("%s: NodeName=%s listed=%s", n.Alias, name, n.Name)
		}
	}
}

func TestNodesErrors(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte("")}, nil)
	if _, err := k.Nodes(context.Background(), "ks-x"); err == nil || !strings.Contains(err.Error(), "doctor") {
		t.Fatalf("empty: %v", err)
	}
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte("something-else\n")}, nil)
	if _, err := k.Nodes(context.Background(), "ks-x"); err == nil {
		t.Fatal("unexpected node name")
	}
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{ExitCode: 1}, nil)
	if _, err := k.Nodes(context.Background(), "ks-x"); err == nil {
		t.Fatal("kind failure")
	}
}

func TestNodeExecAndShell(t *testing.T) {
	k, f := newKind()
	ctx := context.Background()
	if _, err := k.NodeExec(ctx, "ks-x", "node02", nil, "systemctl", "status", "kubelet"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.NodeExec(ctx, "ks-x", "controlplane", strings.NewReader("in"), "cat"); err != nil {
		t.Fatal(err)
	}
	if err := k.NodeShell(ctx, "ks-x", "node01"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker exec ks-x-worker2 systemctl status kubelet",
		"docker exec -i ks-x-control-plane cat",
		"docker exec -it ks-x-worker bash -l",
	}
	for i, w := range want {
		if f.Lines()[i] != w {
			t.Errorf("call %d: got %q want %q", i, f.Lines()[i], w)
		}
	}
	if f.Calls[1].Stdin == nil {
		t.Fatal("stdin must be passed through")
	}
	if _, err := k.NodeExec(ctx, "ks-x", "bogus", nil, "ls"); err == nil {
		t.Fatal("bad alias")
	}
	if err := k.NodeShell(ctx, "ks-x", "bogus"); err == nil {
		t.Fatal("bad alias")
	}
	f.Respond([]string{"docker", "exec"}, execx.Result{}, errors.New("no docker"))
	if _, err := k.NodeExec(ctx, "ks-x", "node01", nil, "ls"); err == nil || !strings.Contains(err.Error(), "docker running") {
		t.Fatalf("err: %v", err)
	}
}

func TestLoadImages(t *testing.T) {
	k, f := newKind()
	ctx := context.Background()
	f.Respond([]string{"docker", "image", "inspect", "missing:1"}, execx.Result{ExitCode: 1}, nil)
	if err := k.LoadImages(ctx, "ks-x", []string{"present:1", "missing:1"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker image inspect present:1",
		"kind load docker-image present:1 --name ks-x",
		"docker image inspect missing:1",
		"docker pull missing:1",
		"kind load docker-image missing:1 --name ks-x",
	}
	if strings.Join(f.Lines(), "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(f.Lines(), "\n"))
	}
	f.Respond([]string{"kind", "load", "docker-image", "bad:1"}, execx.Result{ExitCode: 1, Stderr: []byte("ctr: failed")}, nil)
	f.Respond([]string{"docker", "save"}, execx.Result{ExitCode: 1, Stderr: []byte("save failed")}, nil)
	f.Respond([]string{"docker", "pull", "nopull:1"}, execx.Result{ExitCode: 1, Stderr: []byte("denied")}, nil)
	f.Respond([]string{"docker", "image", "inspect", "nopull:1"}, execx.Result{ExitCode: 1}, nil)
	err := k.LoadImages(ctx, "ks-x", []string{"bad:1", "nopull:1", "present:1"})
	if err == nil || !strings.Contains(err.Error(), "ctr: failed") || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("both failures must be reported: %v", err)
	}
	if last := f.Lines()[len(f.Lines())-1]; last != "kind load docker-image present:1 --name ks-x" {
		t.Fatalf("later images must still load, last call %q", last)
	}
	f.Respond([]string{"docker", "image", "inspect"}, execx.Result{}, errors.New("no docker"))
	if err := k.LoadImages(ctx, "ks-x", []string{"x"}); err == nil {
		t.Fatal("docker missing")
	}
	if New("/s").Name() != "kind" {
		t.Fatal("name")
	}
}

func TestLoadImagesRepullsWhenTheSaveFails(t *testing.T) {
	k, f := newKind()
	ctx := context.Background()
	f.Respond([]string{"kind", "load", "docker-image", "broken:1"}, execx.Result{ExitCode: 1, Stderr: []byte("content digest not found")}, nil)
	f.Once([]string{"docker", "save"}, execx.Result{ExitCode: 1, Stderr: []byte("no suitable export target found")}, nil)
	f.Respond([]string{"docker", "save"}, execx.Result{Stdout: []byte("TARBALL")}, nil)
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte("ks-x-control-plane\n")}, nil)
	if err := k.LoadImages(ctx, "ks-x", []string{"broken:1"}); err != nil {
		t.Fatalf("the repull must rescue the load: %v", err)
	}
	lines := strings.Join(f.Lines(), "\n")
	pull := "docker pull --platform linux/" + runtime.GOARCH + " broken:1"
	if strings.Count(lines, "docker save") != 2 || !strings.Contains(lines, pull) {
		t.Fatalf("want save, %s, save:\n%s", pull, lines)
	}
	if strings.Index(lines, "docker save") > strings.Index(lines, pull) {
		t.Fatalf("the pull must come after the failed save:\n%s", lines)
	}

	// A failing repull is reported and not retried.
	k, f = newKind()
	f.Respond([]string{"kind", "load", "docker-image", "broken:1"}, execx.Result{ExitCode: 1}, nil)
	f.Respond([]string{"docker", "save"}, execx.Result{ExitCode: 1, Stderr: []byte("save failed")}, nil)
	f.Respond([]string{"docker", "pull", "--platform"}, execx.Result{ExitCode: 1, Stderr: []byte("offline")}, nil)
	err := k.LoadImages(ctx, "ks-x", []string{"broken:1"})
	if err == nil || !strings.Contains(err.Error(), "save failed") || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("err: %v", err)
	}
	if strings.Count(strings.Join(f.Lines(), "\n"), "docker save") != 1 {
		t.Fatalf("no second save after a failed pull: %v", f.Lines())
	}
}

func TestLoadImagesFallsBackToSinglePlatformSave(t *testing.T) {
	k, f := newKind()
	ctx := context.Background()
	f.Respond([]string{"kind", "load", "docker-image", "multi:1"}, execx.Result{ExitCode: 1, Stderr: []byte("content digest not found")}, nil)
	f.Respond([]string{"docker", "save"}, execx.Result{Stdout: []byte("TARBALL")}, nil)
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{Stdout: []byte("ks-x-control-plane\nks-x-worker\n")}, nil)
	if err := k.LoadImages(ctx, "ks-x", []string{"multi:1"}); err != nil {
		t.Fatalf("fallback must rescue the load: %v", err)
	}
	var imports []execx.Cmd
	for _, c := range f.Calls {
		if c.Name == "docker" && len(c.Args) > 0 && c.Args[0] == "exec" {
			imports = append(imports, c)
		}
	}
	if len(imports) != 2 || imports[0].Args[3] != "ks-x-control-plane" || imports[1].Args[3] != "ks-x-worker" {
		t.Fatalf("imports: %+v", imports)
	}
	if !strings.Contains(strings.Join(f.Lines(), "\n"), "docker save --platform linux/"+runtime.GOARCH+" multi:1") {
		t.Fatalf("calls: %v", f.Lines())
	}

	f.Respond([]string{"docker", "exec", "--privileged"}, execx.Result{ExitCode: 1, Stderr: []byte("ctr broke")}, nil)
	if err := k.LoadImages(ctx, "ks-x", []string{"multi:1"}); err == nil || !strings.Contains(err.Error(), "ctr broke") {
		t.Fatalf("import failure must surface: %v", err)
	}
	f.Respond([]string{"docker", "exec", "--privileged"}, execx.Result{}, errors.New("no docker"))
	if err := k.LoadImages(ctx, "ks-x", []string{"multi:1"}); err == nil {
		t.Fatal("import run error")
	}
	f.Respond([]string{"kind", "get", "nodes"}, execx.Result{ExitCode: 1}, nil)
	if err := k.LoadImages(ctx, "ks-x", []string{"multi:1"}); err == nil {
		t.Fatal("node listing failure")
	}
}

func TestNodeAccessRefusesForeignClusters(t *testing.T) {
	k, f := newKind()
	if _, err := k.NodeExec(context.Background(), "prod", "node01", nil, "ls"); err == nil {
		t.Fatal("exec must refuse a non ks- cluster")
	}
	if err := k.NodeShell(context.Background(), "prod", "node01"); err == nil {
		t.Fatal("shell must refuse a non ks- cluster")
	}
	if len(f.Calls) != 0 {
		t.Fatal("nothing may run")
	}
}
