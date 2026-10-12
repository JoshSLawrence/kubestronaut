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

func TestAptReleasesAreWellFormed(t *testing.T) {
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for v, r := range aptReleases {
		if _, ok := nodeImages[v]; !ok {
			t.Errorf("aptReleases has %s, which is not a supported version", v)
		}
		if !regexp.MustCompile(`^v1\.\d+$`).MatchString(r.Minor) ||
			!regexp.MustCompile(`^1\.\d+\.\d+-\d+\.\d+$`).MatchString(r.Version) {
			t.Errorf("%s: malformed minor %q or version %q", v, r.Minor, r.Version)
		}
		for arch, sums := range map[string]map[string]string{"amd64": r.SHA256AMD64, "arm64": r.SHA256ARM64} {
			for _, pkg := range []string{"kubeadm", "kubelet", "kubectl"} {
				if !hex64.MatchString(sums[pkg]) {
					t.Errorf("%s %s: no sha256 for %s", v, arch, pkg)
				}
			}
			if len(sums) != 3 {
				t.Errorf("%s %s: want exactly kubeadm, kubelet and kubectl, got %v", v, arch, sums)
			}
		}
	}
	if _, ok := aptReleases["v1.36.4"]; !ok {
		t.Error("v1.36.4 needs its apt release: the kubeadm-upgrade lab upgrades from it")
	}
}

func TestBuildArgsAreDeterministicAndFollowTheAptPin(t *testing.T) {
	a, err := buildArgs("v1.36.4")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if b, _ := buildArgs("v1.36.4"); strings.Join(a, "\n") != strings.Join(b, "\n") {
			t.Fatalf("map order leaked into the build args:\n%v\n%v", a, b)
		}
	}
	if joined := strings.Join(a, "\n"); !strings.Contains(joined, "K8S_APT_MINOR=v1.37") ||
		!strings.Contains(joined, "kubeadm=") || !strings.Contains(joined, " kubelet=") {
		t.Fatalf("apt arguments missing:\n%s", joined)
	}
	if _, err := buildArgs("v0"); err == nil {
		t.Fatal("an unpinned version must fail")
	}

	before, _ := NodeImageTag("v1.36.4")
	saved := aptReleases["v1.36.4"]
	t.Cleanup(func() { aptReleases["v1.36.4"] = saved })
	changed := saved
	changed.SHA256ARM64 = map[string]string{}
	for k, v := range saved.SHA256ARM64 {
		changed.SHA256ARM64[k] = v
	}
	changed.SHA256ARM64["kubelet"] = strings.Repeat("0", 64)
	aptReleases["v1.36.4"] = changed
	if after, _ := NodeImageTag("v1.36.4"); after == before {
		t.Fatal("changing an apt pin must rebuild the node image")
	}
}

func TestNodeFilesForVersionWithAptRepo(t *testing.T) {
	with := strings.Join(nodeFilesFor("v1.36.4"), " ")
	for _, want := range []string{"/opt/kubernetes-apt/v1.37/Packages", "/opt/kubernetes-apt/kubernetes.list", "kind-cni-plugins.list", "bridge"} {
		if !strings.Contains(with, want) {
			t.Errorf("v1.36.4 must be smoke-tested for %s: %s", want, with)
		}
	}
	if without := strings.Join(nodeFilesFor("v1.37.0"), " "); strings.Contains(without, "kubernetes-apt") {
		t.Errorf("v1.37.0 has no apt repository: %s", without)
	}
}

func TestDockerfileApt(t *testing.T) {
	df := string(nodeDockerfile)
	for _, want := range []string{"apt-utils", "ARG K8S_APT_MINOR", "apt-ftparchive packages", "apt-ftparchive release",
		"[trusted=yes]", "Provides: kubernetes-cni", "force-confold"} {
		if !strings.Contains(df, want) {
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

func TestPreparePullsTheLoadBalancerImageForHA(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	var msgs []string
	err := k.Prepare(context.Background(), backend.ClusterSpec{ControlPlanes: 3, NodeImage: "my/img:1"}, func(m string) { msgs = append(msgs, m) })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(f.Lines(), "\n")
	if !strings.Contains(lines, "docker pull "+LoadBalancerImage) {
		t.Fatalf("the load balancer image must be pulled:\n%s", lines)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "load balancer") || !strings.Contains(msgs[0], "internet") {
		t.Fatalf("a slow pull must be announced: %v", msgs)
	}

	// Already local: only inspected, and nothing is announced.
	k, f = newKind()
	msgs = nil
	if err := k.Prepare(context.Background(), backend.ClusterSpec{ControlPlanes: 3, NodeImage: "my/img:1"}, func(m string) { msgs = append(msgs, m) }); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.Lines(), "\n"), "docker pull") || len(msgs) != 0 {
		t.Fatalf("a local image must not be pulled: %v %v", f.Lines(), msgs)
	}

	// One control plane has no load balancer.
	k, f = newKind()
	if err := k.Prepare(context.Background(), backend.ClusterSpec{ControlPlanes: 1, NodeImage: "my/img:1"}, nil); err != nil || len(f.Calls) != 0 {
		t.Fatalf("a single control plane needs no docker call: %v %v", err, f.Lines())
	}
}

func TestPrepareLoadBalancerPullFailureIsActionable(t *testing.T) {
	k, f := newKind()
	f.Respond([]string{"docker", "image", "inspect"}, missingImage, nil)
	f.Respond([]string{"docker", "pull"}, execx.Result{ExitCode: 1, Stderr: []byte("dial tcp: lookup registry-1.docker.io")}, nil)
	err := k.Prepare(context.Background(), backend.ClusterSpec{ControlPlanes: 3, NodeImage: "my/img:1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "internet") || !strings.Contains(err.Error(), "registry-1.docker.io") {
		t.Fatalf("err: %v", err)
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
	// v1.37.0 has no next minor, so the apt arguments are empty.
	wantArgs := "build --build-arg BASE=" + base + " --build-arg ETCD_VERSION=" + etcd.Version +
		" --build-arg ETCD_SHA256_AMD64=" + etcd.SHA256AMD64 + " --build-arg ETCD_SHA256_ARM64=" + etcd.SHA256ARM64 +
		" --build-arg K8S_APT_MINOR= --build-arg K8S_APT_VERSION= --build-arg K8S_APT_SHA256_AMD64= --build-arg K8S_APT_SHA256_ARM64=" +
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
