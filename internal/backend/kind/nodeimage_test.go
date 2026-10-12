package kind

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"regexp"
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
	// Each promised tool maps to the text in the Dockerfile that provides it,
	// so adding to nodeTools without installing it fails here, not at build
	// time. Distribution packages are listed one per line; the rest are
	// downloaded and verified.
	df := string(nodeDockerfile)
	pkgs := map[string]string{
		"vim": "vim", "nano": "nano", "less": "less",
		"dig": "dnsutils", "nslookup": "dnsutils", "ps": "procps", "netstat": "net-tools",
	}
	downloads := map[string]string{
		"etcdctl": "${dir}/etcdctl", "etcdutl": "${dir}/etcdutl", "helm": "linux-${arch}/helm",
	}
	for _, tool := range nodeTools {
		if pkg, ok := pkgs[tool]; ok {
			if !strings.Contains(df, "    "+pkg+" \\") {
				t.Errorf("Dockerfile does not install %s (for %s)", pkg, tool)
			}
			continue
		}
		if marker, ok := downloads[tool]; ok {
			if !strings.Contains(df, marker) {
				t.Errorf("Dockerfile does not extract %s", tool)
			}
			continue
		}
		t.Errorf("no package or download known for tool %q: add it to this test", tool)
	}
	if strings.Contains(df, "etcd-client") {
		t.Error("the Debian etcd-client must not come back: its version does not match the cluster")
	}
	// Every download is checked against a pinned checksum.
	if n, want := strings.Count(df, "curl -fsSL"), strings.Count(df, "sha256sum -c -"); n != want {
		t.Errorf("%d downloads but %d checksum verifications", n, want)
	}
	for _, want := range []string{"ARG BASE", "FROM ${BASE}", "expandtab", "alias k=kubectl", NodeImageLabel,
		"/opt/cni/bin", "./bridge", "/opt/pkgs", "ipvsadm", "ARG ETCD_VERSION"} {
		if !strings.Contains(df, want) {
			t.Errorf("Dockerfile lacks %q", want)
		}
	}
}

func TestEtcdReleasesCoverEveryVersion(t *testing.T) {
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, v := range Versions() {
		r, err := etcdFor(v)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if !regexp.MustCompile(`^3\.\d+\.\d+$`).MatchString(r.Version) ||
			!hex64.MatchString(r.SHA256AMD64) || !hex64.MatchString(r.SHA256ARM64) || r.SHA256AMD64 == r.SHA256ARM64 {
			t.Errorf("%s: malformed etcd pin %+v", v, r)
		}
	}
	if len(etcdReleases) != len(nodeImages) {
		t.Errorf("etcdReleases has %d entries for %d Kubernetes versions", len(etcdReleases), len(nodeImages))
	}
	if _, err := etcdFor("v0"); err == nil || !strings.Contains(err.Error(), "images.go") {
		t.Fatalf("an unpinned version must say where to add it: %v", err)
	}
}

func TestNodeImageTagFollowsEtcdPin(t *testing.T) {
	before, _ := NodeImageTag("v1.36.4")
	saved := etcdReleases["v1.36.4"]
	t.Cleanup(func() { etcdReleases["v1.36.4"] = saved })
	etcdReleases["v1.36.4"] = etcdRelease{"3.6.9", saved.SHA256AMD64, saved.SHA256ARM64}
	if after, _ := NodeImageTag("v1.36.4"); after == before {
		t.Fatal("changing the etcd pin must rebuild the node image")
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
	etcd := etcdReleases["v1.37.0"]
	wantArgs := "build --build-arg BASE=" + base + " --build-arg ETCD_VERSION=" + etcd.Version +
		" --build-arg ETCD_SHA256_AMD64=" + etcd.SHA256AMD64 + " --build-arg ETCD_SHA256_ARM64=" + etcd.SHA256ARM64 +
		" -t " + tag + "-candidate -"
	if got := strings.Join(build.Args, " "); got != wantArgs {
		t.Fatalf("build args: %s", got)
	}
	if b, _ := io.ReadAll(build.Stdin); string(b) != string(nodeDockerfile) {
		t.Fatal("the Dockerfile must go in on stdin")
	}
	args := strings.Join(smoke.Args, " ")
	for _, want := range []string{"vim", "etcdutl", "helm", "/opt/cni/bin/bridge", "ipvsadm", tag + "-candidate"} {
		if !strings.Contains(args, want) {
			t.Fatalf("smoke test must check the candidate for %s: %v", want, smoke.Args)
		}
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
