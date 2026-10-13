package kind

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

// Mount is a read-only bind mount from the Docker host into every node.
type Mount struct {
	HostPath, ContainerPath string
}

// BuildConfig renders the kind cluster config for spec using image.
func BuildConfig(spec backend.ClusterSpec, image string) []byte {
	return buildConfig(spec, image, nil)
}

func buildConfig(spec backend.ClusterSpec, image string, mounts []Mount) []byte {
	var b strings.Builder
	b.WriteString("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")
	if len(spec.KubeadmPatches) > 0 {
		b.WriteString("kubeadmConfigPatches:\n")
		for _, p := range spec.KubeadmPatches {
			// A JSON string is also a valid YAML scalar, which spares us
			// from hand-rolling block scalar indentation.
			q, _ := json.Marshal(p)
			fmt.Fprintf(&b, "  - %s\n", q)
		}
	}
	if spec.DisableDefaultCNI {
		b.WriteString("networking:\n  disableDefaultCNI: true\n")
	}
	b.WriteString("nodes:\n")
	node := func(role string) {
		fmt.Fprintf(&b, "  - role: %s\n    image: %s\n", role, image)
		if len(mounts) == 0 {
			return
		}
		b.WriteString("    extraMounts:\n")
		for _, m := range mounts {
			hp, _ := json.Marshal(m.HostPath)
			cp, _ := json.Marshal(m.ContainerPath)
			fmt.Fprintf(&b, "      - hostPath: %s\n        containerPath: %s\n        readOnly: true\n", hp, cp)
		}
	}
	for i := 0; i < max(spec.ControlPlanes, 1); i++ {
		node("control-plane")
	}
	for i := 0; i < spec.Workers; i++ {
		node("worker")
	}
	return []byte(b.String())
}

// hostMounts returns what Create mounts into every node.
//
// kubeadm's preflight SystemVerification must read the kernel's build
// config, and treats a missing one as fatal. Docker Desktop's VM exposes
// /proc/config.gz, but most Linux kernels (Ubuntu, GitHub's Azure runners)
// only ship /boot/config-<release>, which kind nodes cannot see. On Linux the
// node runs on the host's own kernel, so mounting that one file lets `kubeadm
// join` and `kubeadm upgrade` verify the real kernel instead of failing.
func hostMounts() []Mount {
	return kernelConfigMounts(runtime.GOOS, os.Getenv("DOCKER_HOST"), os.ReadFile, os.Stat)
}

func kernelConfigMounts(goos, dockerHost string, readFile func(string) ([]byte, error),
	stat func(string) (os.FileInfo, error)) []Mount {
	// A remote daemon runs on another kernel, and bind mounts resolve on
	// its filesystem, so the local file would be the wrong one or missing.
	if goos != "linux" || (dockerHost != "" && !strings.HasPrefix(dockerHost, "unix://")) {
		return nil
	}
	rel, err := readFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil
	}
	path := "/boot/config-" + strings.TrimSpace(string(rel))
	if _, err := stat(path); err != nil {
		return nil
	}
	return []Mount{{HostPath: path, ContainerPath: path}}
}
