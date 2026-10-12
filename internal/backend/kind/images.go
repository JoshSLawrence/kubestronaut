package kind

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultVersion is used when a scenario names no Kubernetes version.
const DefaultVersion = "v1.36.4"

// nodeImages pins each supported Kubernetes version to the node image digest
// published in the kind v0.33.0 release notes. Digests keep clusters
// reproducible: a tag alone can be re-pushed.
var nodeImages = map[string]string{
	"v1.35.8": "kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0",
	"v1.36.4": "kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed",
	"v1.37.0": "kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5",
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
