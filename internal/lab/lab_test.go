package lab

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend/fake"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/session"
)

const packYAML = `apiVersion: kubestronaut/v1alpha1
kind: Pack
metadata: {name: cka}
spec:
  title: CKA
  passPercent: 66
  domains: [{id: troubleshooting, title: T, weight: 1}]
`

const scenYAML = `apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata: {name: demo, title: Demo, domain: troubleshooting}
spec:
  timeLimit: 10m
  weight: 7
  context: k8s-demo
  cluster: {workers: 1}
  images: [registry.example/agnhost:1]
  prompt: prompt.md
  hints: [first hint, second hint]
  setup:
    - script: setup/s.sh
    - manifest: setup/m.yaml
  checks:
    - id: pod-exists
      description: pod exists
      type: resourceExists
      ref: {kind: pod, name: p}
  solution:
    script: solution/solve.sh
    readme: solution/README.md
`

type fixture struct {
	lab *Lab
	be  *fake.Fake
	run *execx.Fake
	k   *kube.Fake
	out *bytes.Buffer
	dir string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	fsys := fstest.MapFS{
		"cka/pack.yaml":               {Data: []byte(packYAML)},
		"cka/demo/scenario.yaml":      {Data: []byte(scenYAML)},
		"cka/demo/prompt.md":          {Data: []byte("Fix {{node \"node01\"}}.\n")},
		"cka/demo/setup/s.sh":         {Data: []byte("true\n")},
		"cka/demo/setup/m.yaml":       {Data: []byte("kind: Namespace\n")},
		"cka/demo/solution/solve.sh":  {Data: []byte("true\n")},
		"cka/demo/solution/README.md": {Data: []byte("Do the thing.\n")},
	}
	cat := scenario.LoadCatalog(fsys)
	if len(cat.Problems) != 0 {
		t.Fatal(cat.Problems)
	}
	dir := t.TempDir()
	f := &fixture{be: fake.New(), run: &execx.Fake{}, k: &kube.Fake{}, out: &bytes.Buffer{}, dir: dir}
	f.run.Respond([]string{"kubectl", "config", "view"}, execx.Result{Stdout: []byte("merged")}, nil)
	f.lab = New(cat, f.be, &session.Store{Root: filepath.Join(dir, "home")}, f.run, "/bin/ks", f.out)
	f.lab.KubeFor = func(string) kube.Client { return f.k }
	return f
}

func (f *fixture) bashCalls() []string {
	var out []string
	for _, c := range f.run.Calls {
		if c.Name == "bash" {
			out = append(out, c.Args[0])
		}
	}
	return out
}

func TestStartHappyPath(t *testing.T) {
	f := newFixture(t)
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Created) != 1 {
		t.Fatalf("created: %+v", f.be.Created)
	}
	c := f.be.Created[0]
	if c.Name != "ks-cka-demo" || c.Workers != 1 || c.KubernetesVersion != "v1.36.4" || !strings.HasSuffix(c.Kubeconfig, "admin.kubeconfig") {
		t.Fatalf("spec: %+v", c)
	}
	if got := f.be.Loaded["ks-cka-demo"]; len(got) != 1 {
		t.Fatalf("images: %v", got)
	}
	if got := strings.Join(f.bashCalls(), ","); got != "setup/s.sh" {
		t.Fatalf("scripts: %s", got)
	}
	if len(f.k.Applied) != 1 || string(f.k.Applied[0]) != "kind: Namespace" {
		t.Fatalf("applied: %q", f.k.Applied)
	}
	var scriptCall execx.Cmd
	for _, c := range f.run.Calls {
		if c.Name == "bash" {
			scriptCall = c
		}
	}
	envs := strings.Join(scriptCall.Env, "\n")
	for _, want := range []string{"KS_BIN=/bin/ks", "KS_CLUSTER=ks-cka-demo", "KS_NODE_NODE01=ks-cka-demo-worker", "KUBESTRONAUT_HOME=" + f.lab.Store.Root, "KUBECONFIG="} {
		if !strings.Contains(envs, want) {
			t.Errorf("script env missing %s:\n%s", want, envs)
		}
	}
	out := f.out.String()
	for _, want := range []string{"Context: k8s-demo", "Suggested time: 10m", "Weight: 7%", "Fix ks-cka-demo-worker.", "export KUBECONFIG=" + f.lab.Store.KubeconfigPath()} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	st, err := f.lab.Store.Load()
	if err != nil || len(st.Clusters[0].Nodes) != 2 || st.Tasks[0].Scenario != "cka/demo" {
		t.Fatalf("state: %+v %v", st, err)
	}
	if _, err := os.Stat(filepath.Join(f.lab.Store.SessionDir(st.ID), "setup.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.lab.Store.SessionDir(st.ID), "scenario", "cka", "demo", "solution")); err == nil {
		t.Fatal("solution must not be materialized for the learner")
	}

	// A second start must refuse, and --force must replace.
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("second start: %v", err)
	}
	if err := f.lab.Start(context.Background(), "demo", StartOptions{Force: true, K8sVersion: "v1.35.8"}); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Deleted) != 1 || f.be.Created[1].KubernetesVersion != "v1.35.8" {
		t.Fatalf("force: deleted=%v created=%+v", f.be.Deleted, f.be.Created)
	}
}

func TestStartFailureCleansUp(t *testing.T) {
	f := newFixture(t)
	f.run.Respond([]string{"bash"}, execx.Result{ExitCode: 1, Stderr: []byte("boom")}, nil)
	err := f.lab.Start(context.Background(), "demo", StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "setup.log") {
		t.Fatalf("err: %v", err)
	}
	if len(f.be.Deleted) != 1 {
		t.Fatalf("cluster must be deleted: %v", f.be.Deleted)
	}
	if _, err := f.lab.Store.Load(); !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("state must be cleared: %v", err)
	}
}

func TestStartKeepOnFailure(t *testing.T) {
	f := newFixture(t)
	f.run.Respond([]string{"bash"}, execx.Result{ExitCode: 1}, nil)
	if err := f.lab.Start(context.Background(), "demo", StartOptions{KeepOnFailure: true}); err == nil {
		t.Fatal("want failure")
	}
	if len(f.be.Deleted) != 0 {
		t.Fatal("cluster must be kept")
	}
	if _, err := f.lab.Store.Load(); err != nil {
		t.Fatalf("state must stay so stop works: %v", err)
	}
	if !strings.Contains(f.out.String(), "kept for debugging") {
		t.Fatalf("output: %s", f.out.String())
	}
	if err := f.lab.Stop(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Deleted) != 1 {
		t.Fatal("stop must delete the kept cluster")
	}
}

func TestStartCreateFailureAndInterrupt(t *testing.T) {
	f := newFixture(t)
	f.be.CreateErr = errors.New("docker down")
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err == nil || !strings.Contains(err.Error(), "docker down") {
		t.Fatalf("err: %v", err)
	}
	if _, err := f.lab.Store.Load(); !errors.Is(err, session.ErrNoSession) {
		t.Fatal("state must be cleared")
	}
	// A cancelled context must still delete the half-built cluster.
	f = newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.run.Respond([]string{"bash"}, execx.Result{}, context.Canceled)
	cancel()
	if err := f.lab.Start(ctx, "demo", StartOptions{}); err == nil {
		t.Fatal("want error")
	}
	if len(f.be.Deleted) != 1 {
		t.Fatalf("cleanup must use a detached context: %v", f.be.Deleted)
	}
	if err := f.lab.Start(context.Background(), "nope", StartOptions{}); err == nil {
		t.Fatal("unknown scenario")
	}
}

func TestImageLoadFailureIsOnlyAWarning(t *testing.T) {
	f := newFixture(t)
	f.be.LoadErr = errors.New("no kind load")
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "warning: image preload failed") {
		t.Fatalf("output:\n%s", f.out.String())
	}
}

func TestGradeHintSolutionStatus(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.lab.Grade(ctx); !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("grade without session: %v", err)
	}
	if err := f.lab.Start(ctx, "cka/demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}

	rep, err := f.lab.Grade(ctx)
	if err != nil || rep.Passed || rep.Possible != 1 {
		t.Fatalf("baseline: %+v %v", rep, err)
	}
	f.k.Add(map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "p", "namespace": "default"}})

	h, shown, total, err := f.lab.Hint(false)
	if err != nil || h[0] != "first hint" || shown != 1 || total != 2 {
		t.Fatalf("hint 1: %v %d %d %v", h, shown, total, err)
	}
	if _, _, _, err := f.lab.Hint(false); err != nil {
		t.Fatal(err)
	}
	if h, shown, _, _ = f.lab.Hint(false); h[0] != "second hint" || shown != 2 {
		t.Fatalf("hints must stop at the last: %v %d", h, shown)
	}
	if h, _, _, _ = f.lab.Hint(true); len(h) != 2 {
		t.Fatalf("all hints: %v", h)
	}
	sol, err := f.lab.Solution()
	if err != nil || sol != "Do the thing.\n" {
		t.Fatalf("solution: %q %v", sol, err)
	}

	rep, err = f.lab.Grade(ctx)
	if err != nil || !rep.Passed || rep.HintsUsed != 2 || !rep.SolutionViewed {
		t.Fatalf("graded: %+v %v", rep, err)
	}
	st, err := f.lab.Status()
	if err != nil || st.LastGrade == nil || !st.LastGrade.Passed || st.HintsUsed != 2 || !st.Solution || st.Context != "k8s-demo" {
		t.Fatalf("status: %+v %v", st, err)
	}
	grades, _ := filepath.Glob(filepath.Join(f.lab.Store.SessionDir(mustID(t, f)), "grades", "*.json"))
	if len(grades) == 0 {
		t.Fatal("grade report must be written to grades/")
	}
	p, err := f.lab.Prompt()
	if err != nil || !strings.Contains(p, "Fix ks-cka-demo-worker.") {
		t.Fatalf("prompt: %q %v", p, err)
	}
}

func mustID(t *testing.T, f *fixture) string {
	t.Helper()
	st, err := f.lab.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st.ID
}

func TestElapsedUsesInjectedClock(t *testing.T) {
	f := newFixture(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	f.lab.Now = func() time.Time { return now }
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(7 * time.Minute)
	st, err := f.lab.Status()
	if err != nil || st.Elapsed != 7*time.Minute {
		t.Fatalf("elapsed: %v %v", st.Elapsed, err)
	}
}

func TestHintWithoutHints(t *testing.T) {
	f := newFixture(t)
	f.lab.Catalog.Scenarios[0].Spec.Hints = nil
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := f.lab.Hint(false); err == nil {
		t.Fatal("want error")
	}
}

func TestStopAndReset(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.lab.Stop(ctx, false); !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("stop without session: %v", err)
	}
	if err := f.lab.Start(ctx, "demo", StartOptions{K8sVersion: "v1.35.8"}); err != nil {
		t.Fatal(err)
	}
	if err := f.lab.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Created) != 2 || f.be.Created[1].KubernetesVersion != "v1.35.8" || len(f.be.Deleted) != 1 {
		t.Fatalf("reset: created=%+v deleted=%v", f.be.Created, f.be.Deleted)
	}

	// stop --all also removes clusters unknown to the state.
	f.be.Clusters["ks-orphan"] = fake.Nodes("ks-orphan", 0)
	f.be.Clusters["not-ours"] = fake.Nodes("not-ours", 0)
	if err := f.lab.Stop(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.be.Clusters["ks-orphan"]; ok {
		t.Fatal("orphan must be deleted")
	}
	if _, ok := f.be.Clusters["not-ours"]; !ok {
		t.Fatal("foreign cluster must survive")
	}
	if _, err := f.lab.Store.Load(); !errors.Is(err, session.ErrNoSession) {
		t.Fatal("state must be cleared")
	}
	if err := f.lab.Reset(ctx); !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("reset without session: %v", err)
	}
	// stop --all works with no session at all, and with a corrupt state file.
	if err := f.lab.Stop(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.lab.Store.Root, "state.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.lab.Stop(ctx, false); err == nil {
		t.Fatal("plain stop must report corrupt state")
	}
	if err := f.lab.Stop(ctx, true); err != nil {
		t.Fatalf("stop --all must recover from corrupt state: %v", err)
	}
}

func TestStopKeepsStateWhenDeleteFails(t *testing.T) {
	f := newFixture(t)
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	f.be.DeleteErr = errors.New("docker hiccup")
	err := f.lab.Stop(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("err: %v", err)
	}
	if _, err := f.lab.Store.Load(); err != nil {
		t.Fatal("state must be kept for a retry")
	}
}

func TestRunSolutionAndSSH(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.lab.RunSolution(ctx); !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("no session: %v", err)
	}
	if err := f.lab.Start(ctx, "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.lab.RunSolution(ctx); err != nil {
		t.Fatal(err)
	}
	calls := f.bashCalls()
	if last := calls[len(calls)-1]; !strings.HasSuffix(last, filepath.Join("solution", "solve.sh")) {
		t.Fatalf("solution script: %s", last)
	}
	f.run.Respond([]string{"bash"}, execx.Result{ExitCode: 3, Stderr: []byte("nope")}, nil)
	if err := f.lab.RunSolution(ctx); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("failing solution: %v", err)
	}
	f.run.Respond([]string{"bash"}, execx.Result{}, errors.New("no bash"))
	if err := f.lab.RunSolution(ctx); err == nil {
		t.Fatal("run error")
	}

	f.be.ExecFunc = func(_, _ string, argv []string) (execx.Result, error) {
		return execx.Result{Stdout: []byte(strings.Join(argv, " ")), ExitCode: 4}, nil
	}
	var o, e bytes.Buffer
	code, err := f.lab.SSH(ctx, "", "node01", []string{"ls", "/"}, nil, &o, &e)
	if err != nil || code != 4 || o.String() != "ls /" {
		t.Fatalf("ssh: %d %q %v", code, o.String(), err)
	}
	if got := f.be.Execs[len(f.be.Execs)-1]; got.Cluster != "ks-cka-demo" || got.Alias != "node01" {
		t.Fatalf("target: %+v", got)
	}
	if _, err := f.lab.SSH(ctx, "ks-other", "controlplane", nil, nil, &o, &e); err != nil {
		t.Fatal(err)
	}
	if f.be.Shells[0] != "ks-other/controlplane" {
		t.Fatalf("shell: %v", f.be.Shells)
	}
	f.be.ExecFunc = func(_, _ string, _ []string) (execx.Result, error) { return execx.Result{}, errors.New("gone") }
	if _, err := f.lab.SSH(ctx, "", "node01", []string{"x"}, nil, &o, &e); err == nil {
		t.Fatal("exec error")
	}
}

func TestStartRefusesExistingCluster(t *testing.T) {
	f := newFixture(t)
	f.be.Clusters["ks-cka-demo"] = fake.Nodes("ks-cka-demo", 1)
	err := f.lab.Start(context.Background(), "demo", StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err: %v", err)
	}
	if len(f.be.Deleted) != 0 || len(f.be.Created) != 0 {
		t.Fatalf("a cluster start did not create must never be deleted: deleted=%v", f.be.Deleted)
	}
	if _, ok := f.be.Clusters["ks-cka-demo"]; !ok {
		t.Fatal("pre-existing cluster vanished")
	}
	entries, _ := os.ReadDir(filepath.Join(f.lab.Store.Root, "sessions"))
	if len(entries) != 0 {
		t.Fatalf("no session dir may be left: %v", entries)
	}
}

func TestStartFailureRemovesSessionDir(t *testing.T) {
	f := newFixture(t)
	f.run.Respond([]string{"bash"}, execx.Result{ExitCode: 1}, nil)
	_ = f.lab.Start(context.Background(), "demo", StartOptions{})
	entries, _ := os.ReadDir(filepath.Join(f.lab.Store.Root, "sessions"))
	if len(entries) != 0 {
		t.Fatalf("session dir left behind: %v", entries)
	}
}

func TestStartValidatesFirst(t *testing.T) {
	f := newFixture(t)
	f.lab.Catalog.Scenarios[0].Spec.Solution.Script = ""
	err := f.lab.Start(context.Background(), "demo", StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("err: %v", err)
	}
	if len(f.be.Created) != 0 {
		t.Fatal("no cluster may be created for an invalid scenario")
	}
}

func TestSetupFilesAreRemovedAfterSetup(t *testing.T) {
	f := newFixture(t)
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.lab.Store.SessionDir(mustID(t, f)), "scenario", "cka", "demo")
	for _, gone := range []string{"setup/s.sh", "setup/m.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s must not stay visible to the learner", gone)
		}
	}
	info, err := os.Stat(filepath.Join(f.lab.Store.SessionDir(mustID(t, f)), "setup.log"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("setup.log must be 0600: %v %v", info, err)
	}
}

func TestResetKeepsHelpRecord(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.lab.Start(ctx, "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := f.lab.Hint(false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.lab.Solution(); err != nil {
		t.Fatal(err)
	}
	if err := f.lab.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := f.lab.Store.Load()
	if err != nil || st.Tasks[0].HintsUsed != 1 || !st.Tasks[0].SolutionViewed {
		t.Fatalf("help record lost: %+v %v", st.Tasks, err)
	}
}

func TestNodeBoundsCheck(t *testing.T) {
	f := newFixture(t)
	if err := f.lab.Store.Save(&session.State{Mode: session.ModeScenario, Current: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.lab.Node(""); err == nil {
		t.Fatal("want error for inconsistent state")
	}
}

func TestSetupCleanupKeepsCheckScripts(t *testing.T) {
	f := newFixture(t)
	s := f.lab.Catalog.Scenarios[0]
	// Bypass validation to prove the cleanup itself is safe.
	s.Spec.Setup = append(s.Spec.Setup, scenario.SetupStep{Script: ".", Timeout: scenario.Duration(time.Minute)})
	s.Spec.Setup[0].Script = "setup"
	var raw = []byte(`{"type":"script","path":"setup/check.sh"}`)
	s.Spec.Checks = append(s.Spec.Checks, scenario.CheckSpec{CheckBase: scenario.CheckBase{ID: "scr", Type: "script"}, Raw: raw})
	st := &session.State{ID: "x"}
	dir := f.lab.scenarioDir("x", s)
	if err := os.MkdirAll(filepath.Join(dir, "setup"), 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "setup", "check.sh")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.lab.removeSetupFiles(st, s)
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("check script was deleted: %v", err)
	}
}

func TestStartPreparesNodeImageFirst(t *testing.T) {
	f := newFixture(t)
	f.be.PrepareNotify = "Building node image kubestronaut/node:v1.36.4-abc"
	if err := f.lab.Start(context.Background(), "demo", StartOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Prepared) != 1 || f.be.Prepared[0].Name != f.be.Created[0].Name {
		t.Fatalf("prepare must get the same spec as create: %+v", f.be.Prepared)
	}
	out := f.out.String()
	if b, c := strings.Index(out, "Building node image"), strings.Index(out, "Creating cluster"); b < 0 || c < b {
		t.Fatalf("the build must be announced before the create:\n%s", out)
	}
}

func TestResetKeepsNodeImageOverride(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.lab.Start(ctx, "demo", StartOptions{NodeImage: "my/img:1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.lab.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.be.Created) != 2 || f.be.Created[1].NodeImage != "my/img:1" || f.be.Prepared[1].NodeImage != "my/img:1" {
		t.Fatalf("reset must reuse --node-image: %+v", f.be.Created)
	}
}

func TestStartPrepareFailureCreatesNothing(t *testing.T) {
	f := newFixture(t)
	f.be.PrepareErr = errors.New("no internet")
	err := f.lab.Start(context.Background(), "demo", StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "no internet") {
		t.Fatalf("err: %v", err)
	}
	if len(f.be.Created) != 0 {
		t.Fatal("no cluster may be created when the image is not ready")
	}
	if _, err := f.lab.Store.Load(); !errors.Is(err, session.ErrNoSession) {
		t.Fatal("state must be cleared")
	}
}
