package kind

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

// DefaultVersion is used when a scenario names no Kubernetes version.
const DefaultVersion = backend.DefaultKubernetesVersion

// nodeImages pins each supported Kubernetes version to the node image digest
// published in the kind v0.33.0 release notes. Digests keep clusters
// reproducible: a tag alone can be re-pushed.
var nodeImages = map[string]string{
	"v1.35.8": "kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0",
	"v1.36.4": "kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed",
	"v1.37.0": "kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5",
}

// etcdRelease is an etcd release whose etcdctl and etcdutl go into the node
// image, with the sha256 of its linux tarballs from the release's SHA256SUMS.
type etcdRelease struct {
	Version, SHA256AMD64, SHA256ARM64 string
}

// etcdReleases pins the etcd tools for each supported Kubernetes version to
// the etcd that kubeadm of that version runs (cmd/kubeadm/app/constants), so
// snapshots taken in a lab are read by matching tools.
var etcdReleases = map[string]etcdRelease{
	"v1.35.8": {"3.6.6",
		"887afaa4a99f22d802ccdfbe65730a5e79aa5c9ce2c8799c67e9d804c50ecedb",
		"8a15f5427c111ff4692753682374970fb68878401d946c2c28bdad6857db652f"},
	"v1.36.4": {"3.6.8",
		"cf9cfe91a4856cb90eed9c99e6aee4b708db2c7888b88a6f116281f04b0ea693",
		"438f56a700d17ce761510a3e63e6fa5c1d587b2dd4d7a22c179c09a649366760"},
	"v1.37.0": {"3.7.0",
		"b05cb07f5686dab8f9cdab89986b44f0dd24aaf5c627176aff325e21fa56f9f0",
		"e61a0954fe6a3003ee20f45e773e12e15c58323ea485deb8a55d065170cdafe8"},
}

// Versions lists the supported Kubernetes versions.
func Versions() []string {
	out := make([]string, 0, len(nodeImages))
	for v := range nodeImages {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ResolveImage picks the node image: a raw override wins, otherwise the
// pinned image for version (default DefaultVersion).
func ResolveImage(version, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if version == "" {
		version = DefaultVersion
	}
	img, ok := nodeImages[version]
	if !ok {
		return "", fmt.Errorf("unsupported Kubernetes version %q: use one of %s, or pass --node-image",
			version, strings.Join(Versions(), ", "))
	}
	return img, nil
}
