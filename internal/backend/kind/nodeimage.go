package kind

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// nodeDockerfile layers exam tooling (vim, nano, etcdctl, ...) over the pinned
// kind node image, which ships without any editor.
//
//go:embed nodeimage/Dockerfile
var nodeDockerfile []byte

// NodeImageRepo is the local-only repository built node images are tagged in.
const NodeImageRepo = "kubestronaut/node"

// nodeTools must exist in a built image. Checking them after the build turns a
// silently missing package into a clear error instead of a learner finding
// out halfway through a scenario. Keep it in step with what
// docs/authoring.md promises scenario authors.
var nodeTools = []string{"vim", "nano", "less", "etcdctl", "dig", "nslookup", "ps", "netstat"}

// NodeImageLabel marks every image kubestronaut builds so stale ones can be
// found and pruned.
const NodeImageLabel = "io.kubestronaut.node"

// NodeImageTag is the local tag of the image built for version. The suffix
// hashes the Dockerfile and base image so changing either rebuilds instead of
// reusing a stale image. Package versions are deliberately not hashed: they
// cannot be known offline, and patch-level drift does not matter for practice.
func NodeImageTag(version string) (string, error) {
	base, err := ResolveImage(version, "")
	if err != nil {
		return "", err
	}
	if version == "" {
		version = DefaultVersion
	}
	h := sha256.New()
	h.Write(nodeDockerfile)
	h.Write([]byte{0})
	h.Write([]byte(base))
	return fmt.Sprintf("%s:%s-%s", NodeImageRepo, version, hex.EncodeToString(h.Sum(nil))[:12]), nil
}

// imageExists distinguishes "no such image" from docker itself failing, so a
// stopped daemon is reported as such instead of triggering a doomed build.
func (k *Kind) imageExists(ctx context.Context, ref string) (bool, error) {
	res, err := k.Runner.Run(ctx, execx.Cmd{Name: "docker", Args: []string{"image", "inspect", "--format", "{{.Id}}", ref}})
	if err != nil {
		return false, fmt.Errorf("docker image inspect: %w (is docker installed and on PATH?)", err)
	}
	if res.ExitCode == 0 {
		return true, nil
	}
	if strings.Contains(strings.ToLower(string(res.Stderr)), "no such image") {
		return false, nil
	}
	return false, fmt.Errorf("docker image inspect failed (exit %d): %s: is the Docker daemon running?",
		res.ExitCode, strings.TrimSpace(string(res.Stderr)))
}

// NodeImageBuilt reports whether the node image for version exists locally.
func (k *Kind) NodeImageBuilt(ctx context.Context, version string) (string, bool, error) {
	tag, err := NodeImageTag(version)
	if err != nil {
		return "", false, err
	}
	ok, err := k.imageExists(ctx, tag)
	return tag, ok, err
}

// StaleNodeImages lists kubestronaut node images that no supported version
// uses any more, for doctor to report.
func (k *Kind) StaleNodeImages(ctx context.Context) ([]string, error) {
	res, err := k.run(ctx, "docker", "image", "ls", "--filter", "label="+NodeImageLabel,
		"--format", "{{.Repository}}:{{.Tag}}")
	if err != nil {
		return nil, err
	}
	current := map[string]bool{}
	for _, v := range Versions() {
		if tag, err := NodeImageTag(v); err == nil {
			current[tag] = true
		}
	}
	var stale []string
	for _, l := range strings.Split(string(res.Stdout), "\n") {
		if l = strings.TrimSpace(l); l != "" && !current[l] {
			stale = append(stale, l)
		}
	}
	return stale, nil
}

// Prepare implements backend.Backend by building the node image the first
// time a Kubernetes version is used. An explicit --node-image is used as is.
func (k *Kind) Prepare(ctx context.Context, s backend.ClusterSpec, notify func(string)) error {
	if s.NodeImage != "" {
		return nil
	}
	tag, ok, err := k.NodeImageBuilt(ctx, s.KubernetesVersion)
	if err != nil || ok {
		return err
	}
	if notify != nil {
		notify(fmt.Sprintf("Building node image %s (once per Kubernetes version; needs internet)", tag))
	}
	_, err = k.buildNodeImage(ctx, s.KubernetesVersion)
	return err
}

// buildNodeImage returns the node image tag for version, building it if it is
// missing. Create calls it as well as Prepare, so it must be cheap when the
// image already exists.
func (k *Kind) buildNodeImage(ctx context.Context, version string) (string, error) {
	tag, ok, err := k.NodeImageBuilt(ctx, version)
	if err != nil || ok {
		return tag, err
	}
	base, err := ResolveImage(version, "")
	if err != nil {
		return "", err
	}
	// Build under a candidate tag and promote it only after the smoke test,
	// so a broken image never sits under the real tag where later starts
	// would trust it without testing.
	candidate := tag + "-candidate"
	// The Dockerfile goes in on stdin with no build context: it copies no
	// files, and an empty context keeps the build independent of the cwd.
	// No --load: the legacy builder rejects it. A non-default buildx builder
	// is caught by the smoke test below instead.
	res, err := k.Runner.Run(ctx, execx.Cmd{
		Name:  "docker",
		Args:  []string{"build", "--build-arg", "BASE=" + base, "-t", candidate, "-"},
		Stdin: bytes.NewReader(nodeDockerfile),
	})
	if err != nil {
		return "", fmt.Errorf("docker build: %w (is docker installed and on PATH?)", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("building node image %s failed (exit %d): %s\n"+
			"The first build per Kubernetes version needs a running Docker daemon and internet access to install "+
			"packages; check both, or pass --node-image to use an image you built yourself",
			tag, res.ExitCode, lastLines(res.Stderr, 15))
	}
	defer func() {
		// Best effort: the candidate is only an intermediate name. A detached
		// context lets it go even when ctx was cancelled.
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		_, _ = k.Runner.Run(cctx, execx.Cmd{Name: "docker", Args: []string{"rmi", candidate}})
	}()
	check := "for t in " + strings.Join(nodeTools, " ") + `; do command -v "$t" >/dev/null || echo "$t"; done`
	res, err = k.Runner.Run(ctx, execx.Cmd{Name: "docker", Args: []string{"run", "--rm", "--entrypoint", "bash", candidate, "-c", check}})
	if err != nil {
		return "", fmt.Errorf("docker run: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("could not run the freshly built node image (exit %d): %s\n"+
			"If your active buildx builder is not the default one, run `docker buildx use default` and retry",
			res.ExitCode, lastLines(res.Stderr, 5))
	}
	if missing := strings.Fields(string(res.Stdout)); len(missing) > 0 {
		return "", fmt.Errorf("the node image built from %s lacks %v: add the package to "+
			"internal/backend/kind/nodeimage/Dockerfile", base, missing)
	}
	if _, err := k.run(ctx, "docker", "tag", candidate, tag); err != nil {
		return "", err
	}
	return tag, nil
}

func lastLines(b []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
