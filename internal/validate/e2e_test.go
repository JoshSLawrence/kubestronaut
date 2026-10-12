package validate

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JoshSLawrence/kubestronaut/internal/backend/fake"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/grade"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

const pack = `apiVersion: kubestronaut/v1alpha1
kind: Pack
metadata: {name: cka}
spec:
  title: CKA
  domains: [{id: troubleshooting, title: T, weight: 1}]
`

func scen(extraValidation string) string {
	return `apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata: {name: demo, title: Demo, domain: troubleshooting}
spec:
  context: k8s-demo
  prompt: prompt.md
  checks:
    - id: pod-exists
      description: pod exists
      type: resourceExists
      ref: {kind: pod, name: p, namespace: ns}
    - id: no-bad
      description: no bad pod
      type: resourceExists
      absent: true
      ref: {kind: pod, name: bad, namespace: ns}
  solution:
    script: solution/solve.sh
  validation:
` + extraValidation
}

func catalog(validation string) *scenario.Catalog {
	return scenario.LoadCatalog(fstest.MapFS{
		"cka/pack.yaml":              {Data: []byte(pack)},
		"cka/demo/scenario.yaml":     {Data: []byte(scen(validation))},
		"cka/demo/prompt.md":         {Data: []byte("task")},
		"cka/demo/solution/solve.sh": {Data: []byte("true")},
	})
}

const (
	podJSON  = `{"kind":"Pod","metadata":{"name":"p","namespace":"ns"}}`
	notFound = `Error from server (NotFound): pods "p" not found`
)

// cluster simulates kubectl for the demo scenario: pod p exists only after the
// solution ran, and pod bad exists when badAfter is set and the solution ran.
type cluster struct {
	execx.Fake
	solved       bool
	podAlways    bool
	badAfter     bool
	solutionFail bool
}

func (c *cluster) Run(ctx context.Context, cmd execx.Cmd) (execx.Result, error) {
	switch {
	case cmd.Name == "bash" && strings.HasSuffix(cmd.Args[0], "solve.sh"):
		c.solved = true
		if c.solutionFail {
			return execx.Result{ExitCode: 1, Stderr: []byte("solution broke")}, nil
		}
	case cmd.Name == "kubectl" && slicesContains(cmd.Args, "get"):
		exists := c.podAlways || c.solved
		if slicesContains(cmd.Args, "bad") {
			exists = c.badAfter && c.solved
		}
		if exists {
			return execx.Result{Stdout: []byte(podJSON)}, nil
		}
		return execx.Result{ExitCode: 1, Stderr: []byte(notFound)}, nil
	case cmd.Name == "kubectl" && slicesContains(cmd.Args, "view"):
		return execx.Result{Stdout: []byte("merged")}, nil
	}
	return c.Fake.Run(ctx, cmd)
}

func slicesContains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

func options(t *testing.T, cat *scenario.Catalog, r execx.Runner, be *fake.Fake) (Options, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return Options{Catalog: cat, Backend: be, Runner: r, Binary: "/bin/ks", HomeParent: t.TempDir(), Out: &out}, &out
}

func TestStatic(t *testing.T) {
	cat := catalog("    timeout: 5m\n")
	if errs := Static(cat, nil); len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if errs := Static(cat, []string{"demo"}); len(errs) != 0 {
		t.Fatalf("by id: %v", errs)
	}
	if errs := Static(cat, []string{"ghost"}); len(errs) != 1 {
		t.Fatalf("unknown id: %v", errs)
	}
	bad := scenario.LoadCatalog(fstest.MapFS{
		"cka/pack.yaml":          {Data: []byte(pack)},
		"cka/demo/scenario.yaml": {Data: []byte(strings.Replace(scen(""), "type: resourceExists", "type: nope", 1))},
		"cka/demo/prompt.md":     {Data: []byte("task")},
	})
	if errs := Static(bad, nil); len(errs) < 2 {
		t.Fatalf("want solution and check type errors, got %v", errs)
	}
}

func TestE2ESuccess(t *testing.T) {
	// no-bad passes before the solution (pod bad never exists), which is
	// allowed only because it is listed in passBeforeSolution.
	cat := catalog("    passBeforeSolution: [no-bad]\n")
	be := fake.New()
	o, out := options(t, cat, &cluster{}, be)
	res := E2E(context.Background(), o, "demo")
	if !res.OK() {
		t.Fatalf("err: %v\n%s", res.Err, out.String())
	}
	var names []string
	for _, s := range res.Steps {
		names = append(names, s.Name)
	}
	if len(names) != 5 || names[0] != "start" || names[4] != "stop" {
		t.Fatalf("steps: %v", names)
	}
	if len(be.Deleted) != 1 || len(be.Clusters) != 0 {
		t.Fatalf("cluster must be gone: deleted=%v", be.Deleted)
	}
	var b bytes.Buffer
	Print(&b, res)
	if !strings.Contains(b.String(), "PASS  cka/demo") {
		t.Fatalf("print: %s", b.String())
	}
	if HomeParent("/h") != "/h/validate" {
		t.Fatal("HomeParent")
	}
	if entries, _ := os.ReadDir(o.HomeParent); len(entries) != 0 {
		t.Fatalf("isolated state dir must be removed: %v", entries)
	}
}

func TestE2EBaselineMustFail(t *testing.T) {
	cat := catalog("")
	be := fake.New()
	o, _ := options(t, cat, &cluster{podAlways: true}, be)
	res := E2E(context.Background(), o, "demo")
	if res.OK() || !strings.Contains(res.Err.Error(), "already passes before the solution") {
		t.Fatalf("err: %v", res.Err)
	}
	if len(be.Deleted) != 1 {
		t.Fatalf("cluster must be deleted on failure: %v", be.Deleted)
	}
	var b bytes.Buffer
	Print(&b, res)
	if !strings.Contains(b.String(), "FAIL  cka/demo") {
		t.Fatalf("print: %s", b.String())
	}
}

func TestE2EFinalGradeMustBeFull(t *testing.T) {
	// badAfter makes "no-bad" fail after the solution.
	cat := catalog("    passBeforeSolution: [no-bad]\n")
	o, _ := options(t, cat, &cluster{badAfter: true}, fake.New())
	res := E2E(context.Background(), o, "demo")
	if res.OK() || !strings.Contains(res.Err.Error(), "after the solution 1 check(s) still fail (no-bad)") {
		t.Fatalf("err: %v", res.Err)
	}
}

func TestE2ESolutionFailureAndKeep(t *testing.T) {
	cat := catalog("    passBeforeSolution: [no-bad]\n")
	be := fake.New()
	o, out := options(t, cat, &cluster{solutionFail: true}, be)
	res := E2E(context.Background(), o, "demo")
	if res.OK() || !strings.Contains(res.Err.Error(), "solution broke") {
		t.Fatalf("err: %v", res.Err)
	}
	o.Keep = true
	be2 := fake.New()
	o.Backend = be2
	res = E2E(context.Background(), o, "demo")
	if len(be2.Deleted) != 0 || !strings.Contains(out.String(), "kept cluster") {
		t.Fatalf("keep: deleted=%v out=%s", be2.Deleted, out.String())
	}
	_ = res
}

func TestE2ERefusesExistingClusters(t *testing.T) {
	be := fake.New()
	be.Clusters["ks-other"] = fake.Nodes("ks-other", 0)
	o, _ := options(t, catalog(""), &cluster{}, be)
	res := E2E(context.Background(), o, "demo")
	if res.OK() || !strings.Contains(res.Err.Error(), "stop --all") {
		t.Fatalf("err: %v", res.Err)
	}
	if res := E2E(context.Background(), o, "ghost"); res.OK() {
		t.Fatal("unknown scenario")
	}
	bad := scenario.LoadCatalog(fstest.MapFS{
		"cka/pack.yaml":          {Data: []byte(pack)},
		"cka/demo/scenario.yaml": {Data: []byte(scen(""))},
		"cka/demo/prompt.md":     {Data: []byte("task")},
	})
	o.Catalog = bad
	if res := E2E(context.Background(), o, "demo"); res.OK() || !strings.Contains(res.Err.Error(), "static validation") {
		t.Fatalf("err: %v", res.Err)
	}
}

func TestCheckBaselineInfraError(t *testing.T) {
	s := &scenario.Scenario{}
	err := checkBaseline(s, grade.Report{Checks: []grade.CheckOutcome{{ID: "a", Err: "api down"}}})
	if err == nil || !strings.Contains(err.Error(), "errored") {
		t.Fatalf("err: %v", err)
	}
	if ids := notPassing(grade.Report{}); len(ids) != 1 {
		t.Fatalf("empty report must not count as passing: %v", ids)
	}
}
