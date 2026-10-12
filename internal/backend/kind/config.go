package kind

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

// BuildConfig renders the kind cluster config for spec using image.
func BuildConfig(spec backend.ClusterSpec, image string) []byte {
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
	b.WriteString("nodes:\n")
	for i := 0; i < max(spec.ControlPlanes, 1); i++ {
		fmt.Fprintf(&b, "  - role: control-plane\n    image: %s\n", image)
	}
	for i := 0; i < spec.Workers; i++ {
		fmt.Fprintf(&b, "  - role: worker\n    image: %s\n", image)
	}
	return []byte(b.String())
}
