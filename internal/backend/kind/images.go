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

// aptRelease is the next minor's kubeadm, kubelet and kubectl packages, served
// from a local apt repository in the node image so a kubeadm upgrade lab runs
// the real `apt-get install` flow without internet. The sha256s are those of
// the repository's signed Packages index; its InRelease signature was checked
// against key DE15B14486CD377B9E876E1A234654DA9A296436 (isv:kubernetes OBS
// Project) on 2026-10-12. kubernetes-cni and cri-tools are not shipped: the
// image provides a stand-in for the first (kind's own CNI plugins) and nothing
// needs the second.
type aptRelease struct {
	Minor, Version string
	// SHA256 maps a package to its deb's sha256, per architecture.
	SHA256AMD64, SHA256ARM64 map[string]string
}

// aptReleases pins the next-minor packages for the Kubernetes versions that
// have a published next minor and a lab that upgrades to it.
var aptReleases = map[string]aptRelease{
	"v1.36.4": {Minor: "v1.37", Version: "1.37.1-1.1",
		SHA256AMD64: map[string]string{
			"kubeadm": "b7f399d83fcb8c619927042b4dcd06ba68757fb74f9d2baabc358a0e22032f92",
			"kubectl": "f2a1d6e1d2adf5c5c80d3518917a28b66a423698a553c4be4d181df8ddbdde94",
			"kubelet": "a0692776f7f026ff17de0ede1b97f11c657300f586cc7939766c395f64c2e6d9",
		},
		SHA256ARM64: map[string]string{
			"kubeadm": "25d485eade81e05d63000ed2848781fd0127218c6e0e9ca66bfa62d39d41886e",
			"kubectl": "4e6ab8199a5ce617286630fe191abb3ed60ee05e347df919911703587caa870a",
			"kubelet": "c4337289f95727dbe8dea323570acc85969dc06d5c9fd531c3bc711ff4d0ce88",
		}},
}

// pairs renders a package to sha256 map as the sorted "pkg=sha256" list the
// Dockerfile takes, so the build arguments are deterministic.
func pairs(m map[string]string) string {
	out := make([]string, 0, len(m))
	for pkg, sum := range m {
		out = append(out, pkg+"="+sum)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
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
