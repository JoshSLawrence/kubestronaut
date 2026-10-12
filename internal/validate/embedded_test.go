package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/scenarios"
)

// A broken built-in scenario must fail `go test`, not a learner's session.
func TestEmbeddedPacksAreValid(t *testing.T) {
	cat := scenario.LoadCatalog(scenarios.FS)
	if len(cat.Scenarios) == 0 {
		t.Fatal("no embedded scenarios found")
	}
	for _, err := range Static(cat, nil) {
		t.Error(err)
	}
}

// knownKinds are the kinds scenarios may read. kubectl reports a kind that
// the cluster does not know as "no resource type", which reads as zero
// objects so that operator scenarios grade cleanly before the CRDs exist. The
// price is that a typo in a ref reads the same way, so this list is the typo
// guard: a new kind has to be added here on purpose.
var knownKinds = map[string]bool{
	"apiservice": true, "certificate": true, "certificatesigningrequest": true,
	"clusterissuer": true, "clusterrole": true, "clusterrolebinding": true,
	"configmap": true, "customresourcedefinition": true, "daemonset": true,
	"deployment": true, "endpointslice": true, "horizontalpodautoscaler": true,
	"limitrange": true, "networkpolicy": true, "node": true, "persistentvolume": true,
	"persistentvolumeclaim": true, "pod": true, "poddisruptionbudget": true,
	"priorityclass": true, "resourcequota": true, "role": true, "rolebinding": true,
	"secret": true, "service": true, "serviceaccount": true, "storageclass": true,
}

func TestEmbeddedKindsAreKnown(t *testing.T) {
	cat := scenario.LoadCatalog(scenarios.FS)
	for _, s := range cat.Scenarios {
		kinds := map[string]bool{}
		for _, st := range s.Spec.Setup {
			if st.Wait != nil {
				kinds[st.Wait.Ref.Kind] = true
			}
		}
		for _, c := range s.Spec.Checks {
			var peek struct {
				Ref struct {
					Kind string `json:"kind"`
				} `json:"ref"`
			}
			if json.Unmarshal(c.Raw, &peek) == nil && peek.Ref.Kind != "" {
				kinds[peek.Ref.Kind] = true
			}
			// connectivity checks name their source pod in from.
			var conn struct {
				From struct {
					Kind string `json:"kind"`
				} `json:"from"`
			}
			if json.Unmarshal(c.Raw, &conn) == nil && conn.From.Kind != "" {
				kinds[conn.From.Kind] = true
			}
		}
		for k := range kinds {
			if !knownKinds[strings.ToLower(k)] {
				t.Errorf("%s: kind %q is not in knownKinds: fix the typo, or add the kind there", s.ID, k)
			}
		}
	}
}
