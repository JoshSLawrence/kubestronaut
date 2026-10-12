package validate

import (
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
