package kind

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

var missingImage = execx.Result{ExitCode: 1, Stderr: []byte("No such image")}

func TestNodeImageTag(t *testing.T) {
	a, err := NodeImageTag("v1.36.4")
	if err != nil {
		t.Fatal(err)
	}
	if def, _ := NodeImageTag(""); def != a {
		t.Fatalf("empty version must mean %s: %s vs %s", DefaultVersion, def, a)
	}
	if !strings.HasPrefix(a, NodeImageRepo+":v1.36.4-") {
		t.Fatalf("tag %q", a)
	}
	if b, _ := NodeImageTag("v1.37.0"); b == a || !strings.HasPrefix(b, NodeImageRepo+":v1.37.0-") {
		t.Fatalf("each version needs its own tag: %q", b)
	}
	if _, err := NodeImageTag("v0"); err == nil {
		t.Fatal("unknown version must fail")
	}
}

func TestDockerfileInstallsExamTools(t *testing.T) {
	// Each promised tool maps to the package that provides it, so adding to
	// nodeTools without a matching package fails here, not at build time.
	pkgs := map[string]string{
		"vim": "vim", "nano": "nano", "less": "less", "etcdctl": "etcd-client",
		"dig": "dnsutils", "nslookup": "dnsutils", "ps": "procps", "netstat": "net-tools",
	}
	for _, tool := range nodeTools {
		pkg, ok := pkgs[tool]
		if !ok {
			t.Errorf("no package known for tool %q: add it to this test", tool)
			continue
		}
		if !strings.Contains(string(nodeDockerfile), "    "+pkg+" \\") {
			t.Errorf("Dockerfile does not install %s (for %s)", pkg, tool)
		}
	}
	for _, want := range []string{"ARG BASE", "FROM ${BASE}", "expandtab", "alias k=kubectl", NodeImageLabel} {
		if !strings.Contains(string(nodeDockerfile), want) {
			t.Errorf("Dockerfile lacks %q", want)
		}
	}
}

func TestPrepareSkipsExistingAndOverride(t *testing.T) {
	k, f := newKind()
	notified := false
	notify := func(string) { notified = true }
	if err := k.Prepare(context.Background(), backend.ClusterSpec{NodeImage: "my/img:1"}, notify); err != nil {
		t.Fatal(err)
	}
	if len(f.Calls) != 0 {
		t.Fatalf("an override needs no docker calls: %v", f.Lines())
	}
	// The fake answers exit 0 by default, so the image already exists.
	if err := k.Prepare(context.Background(), backend.ClusterSpec{}, notify); err != nil {
		t.Fatal(err)
	}
	if notified || len(f.Calls) != 1 || !strings.HasPrefix(f.Lines()[0], "docker image inspect") {
		t.Fatalf("existing image must only be inspected: %v notified=%v", f.Lines(), notified)
	}
}

func TestPrepareBuildsAndSmokeTests(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	var msg string
	if err := k.Prepare(context.Background(), backend.ClusterSpec{KubernetesVersion: "v1.37.0"}, func(m string) { msg = m }); err != nil {
		t.Fatal(err)
	}
	tag, _ := NodeImageTag("v1.37.0")
	if !strings.Contains(msg, tag) || !strings.Contains(msg, "internet") {
		t.Fatalf("notify must name the tag and the network need: %q", msg)
	}
	var build, smoke *execx.Cmd
	for i, c := range f.Calls {
		switch {
		case len(c.Args) > 0 && c.Args[0] == "build":
			build = &f.Calls[i]
		case len(c.Args) > 0 && c.Args[0] == "run":
			smoke = &f.Calls[i]
		}
	}
	if build == nil || smoke == nil {
		t.Fatalf("want a build and a smoke test: %v", f.Lines())
	}
	base, _ := ResolveImage("v1.37.0", "")
	if got := strings.Join(build.Args, " "); got != "build --build-arg BASE="+base+" -t "+tag+"-candidate -" {
		t.Fatalf("build args: %s", got)
	}
	if b, _ := io.ReadAll(build.Stdin); string(b) != string(nodeDockerfile) {
		t.Fatal("the Dockerfile must go in on stdin")
	}
	if args := strings.Join(smoke.Args, " "); !strings.Contains(args, "vim") || !strings.Contains(args, tag+"-candidate") {
		t.Fatalf("smoke test must check the candidate for vim: %v", smoke.Args)
	}
	lines := strings.Join(f.Lines(), "\n")
	if !strings.Contains(lines, "docker tag "+tag+"-candidate "+tag) || !strings.Contains(lines, "docker rmi "+tag+"-candidate") {
		t.Fatalf("candidate must be promoted then removed:\n%s", lines)
	}
}

func TestPrepareReportsBuildFailure(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	f.Respond([]string{"docker", "build"}, execx.Result{ExitCode: 1, Stderr: []byte("Temporary failure resolving deb.debian.org")}, nil)
	err := k.Prepare(context.Background(), backend.ClusterSpec{}, nil)
	if err == nil || !strings.Contains(err.Error(), "deb.debian.org") || !strings.Contains(err.Error(), "--node-image") {
		t.Fatalf("err: %v", err)
	}
}

func TestPrepareReportsMissingTools(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	f.Respond([]string{"docker", "run"}, execx.Result{Stdout: []byte("vim\n")}, nil)
	err := k.Prepare(context.Background(), backend.ClusterSpec{}, nil)
	if err == nil || !strings.Contains(err.Error(), "[vim]") || !strings.Contains(err.Error(), "Dockerfile") {
		t.Fatalf("err: %v", err)
	}
	assertNotPromoted(t, f)
}

func TestPrepareSmokeRunFailureHintsAtBuilder(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	f.Respond([]string{"docker", "run"}, execx.Result{ExitCode: 125, Stderr: []byte("Unable to find image")}, nil)
	err := k.Prepare(context.Background(), backend.ClusterSpec{}, nil)
	if err == nil || !strings.Contains(err.Error(), "docker buildx use default") {
		t.Fatalf("err: %v", err)
	}
	assertNotPromoted(t, f)
}

// assertNotPromoted checks a failed smoke test never tags the real image,
// which later starts would trust, and still cleans up the candidate.
func assertNotPromoted(t *testing.T, f *execx.Fake) {
	t.Helper()
	lines := strings.Join(f.Lines(), "\n")
	if strings.Contains(lines, "docker tag") {
		t.Fatalf("a failed image must not be promoted:\n%s", lines)
	}
	if !strings.Contains(lines, "docker rmi") {
		t.Fatalf("the candidate must be removed:\n%s", lines)
	}
}

func TestPrepareDaemonDownIsNotABuild(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, execx.Result{ExitCode: 1,
		Stderr: []byte("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")}, nil)
	err := k.Prepare(context.Background(), backend.ClusterSpec{}, func(string) { t.Fatal("must not announce a build") })
	if err == nil || !strings.Contains(err.Error(), "daemon running") {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(strings.Join(f.Lines(), "\n"), "docker build") {
		t.Fatal("no build when docker itself is down")
	}
}

func TestStaleNodeImages(t *testing.T) {
	k, f := newKind()
	cur, _ := NodeImageTag("v1.36.4")
	f.Respond([]string{"docker", "image", "ls"}, execx.Result{Stdout: []byte(cur + "\n" + NodeImageRepo + ":v1.36.4-000000000000\n")}, nil)
	stale, err := k.StaleNodeImages(context.Background())
	if err != nil || len(stale) != 1 || stale[0] != NodeImageRepo+":v1.36.4-000000000000" {
		t.Fatalf("stale: %v %v", stale, err)
	}
	if !strings.Contains(f.Lines()[0], "label="+NodeImageLabel) {
		t.Fatalf("must filter by label: %s", f.Lines()[0])
	}
}

func TestPrepareDockerMissing(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker"}, execx.Result{}, errors.New("executable file not found"))
	if err := k.Prepare(context.Background(), backend.ClusterSpec{}, nil); err == nil || !strings.Contains(err.Error(), "PATH") {
		t.Fatalf("err: %v", err)
	}
}

func TestCreateBuildsWhenNotPrepared(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	kc := filepath.Join(t.TempDir(), "kc")
	if err := k.Create(context.Background(), backend.ClusterSpec{Name: "ks-x", Kubeconfig: kc}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(f.Lines(), "\n")
	if !strings.Contains(lines, "docker build") || !strings.Contains(lines, "kind create cluster") {
		t.Fatalf("calls: %s", lines)
	}
}
