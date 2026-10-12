//go:build e2e

// Package e2e runs the same end-to-end validation as `kubestronaut validate
// --e2e`, against real kind clusters. It is opt-in because it needs docker,
// kind and kubectl and takes minutes:
//
//	KS_E2E=1 go test -tags e2e -timeout 90m ./test/e2e/
//
// KS_E2E_SCENARIOS (comma separated ids) narrows the run, and
// KS_E2E_TIMEOUT sets the go test timeout used by `mise run e2e`.
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend/kind"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/validate"
	"github.com/JoshSLawrence/kubestronaut/scenarios"
)

// ids defaults to every non-experimental scenario in the catalog, so new
// scenarios and packs are covered without editing this file.
func ids(cat *scenario.Catalog) []string {
	v := strings.TrimSpace(os.Getenv("KS_E2E_SCENARIOS"))
	if v == "" {
		var all []string
		for _, s := range cat.Scenarios {
			if !s.Metadata.Experimental {
				all = append(all, s.ID)
			}
		}
		return all
	}
	var out []string
	for _, id := range strings.Split(v, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// buildBinary compiles the CLI because scenario scripts call it back through
// $KS_BIN (ks ssh), and the test binary itself is not that CLI.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kubestronaut")
	res, err := execx.OS{}.Run(context.Background(), execx.Cmd{
		Name: "go", Args: []string{"build", "-o", bin, "github.com/JoshSLawrence/kubestronaut/cmd/kubestronaut"},
	})
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("build kubestronaut: %v\n%s", err, res.Stderr)
	}
	return bin
}

func TestScenarios(t *testing.T) {
	if os.Getenv("KS_E2E") != "1" {
		t.Skip("set KS_E2E=1 to run end-to-end tests against real kind clusters")
	}
	bin := buildBinary(t)
	cat := scenario.LoadCatalog(scenarios.FS)
	home := t.TempDir()
	be := kind.New(filepath.Join(home, "scratch.kubeconfig"))

	for _, id := range ids(cat) {
		t.Run(strings.ReplaceAll(id, "/", "_"), func(t *testing.T) {
			s, err := cat.Find(id)
			if err != nil {
				t.Fatal(err)
			}
			name := scenario.ClusterName(s.Pack, s.DirName)
			// Never adopt a cluster this test did not create: E2E refuses to
			// run next to existing clusters, and the cleanup below must not
			// delete one that was there before.
			existing, err := be.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(existing, name) {
				t.Fatalf("cluster %s already exists: run `kubestronaut stop --all` first", name)
			}
			t.Cleanup(func() {
				if err := be.Delete(context.Background(), name); err != nil {
					t.Logf("cleanup: %v", err)
				}
			})
			res := validate.E2E(context.Background(), validate.Options{
				Catalog: cat, Backend: be, Runner: execx.OS{}, Binary: bin,
				HomeParent: filepath.Join(home, "validate"), Out: os.Stdout, Verbose: testing.Verbose(),
			}, id)
			validate.Print(os.Stdout, res)
			if !res.OK() {
				t.Fatal(res.Err)
			}
		})
	}
}
