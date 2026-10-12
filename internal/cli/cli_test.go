package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JoshSLawrence/kubestronaut/internal/backend/fake"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

const pack = `apiVersion: kubestronaut/v1alpha1
kind: Pack
metadata: {name: cka}
spec:
  title: CKA
  domains: [{id: troubleshooting, title: T, weight: 1}, {id: storage, title: S, weight: 1}]
`

const scen = `apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata: {name: demo, title: Demo scenario, domain: troubleshooting, difficulty: easy, tags: [a, b]}
spec:
  timeLimit: 10m
  weight: 7
  context: k8s-demo
  prompt: prompt.md
  hints: [first, second]
  checks:
    - id: pod-exists
      description: pod exists
      type: resourceExists
      ref: {kind: pod, name: p, namespace: ns}
  solution:
    script: solution/solve.sh
    readme: solution/README.md
`

func layersOf(m fstest.MapFS) []fs.FS { return []fs.FS{m} }

type env struct {
	app *App
	run *execx.Fake
	be  *fake.Fake
}

func newEnv(t *testing.T) *env {
	t.Helper()
	exp := strings.Replace(strings.Replace(scen, "name: demo", "name: beta", 1), "domain: troubleshooting", "domain: storage", 1)
	exp = strings.Replace(exp, "difficulty: easy", "difficulty: easy, experimental: true", 1)
	m := fstest.MapFS{
		"cka/pack.yaml":               {Data: []byte(pack)},
		"cka/demo/scenario.yaml":      {Data: []byte(scen)},
		"cka/demo/prompt.md":          {Data: []byte("Do the task.\n")},
		"cka/demo/solution/solve.sh":  {Data: []byte("true")},
		"cka/demo/solution/README.md": {Data: []byte("The answer.\n")},
		"cka/beta/scenario.yaml":      {Data: []byte(exp)},
		"cka/beta/prompt.md":          {Data: []byte("x")},
		"cka/beta/solution/solve.sh":  {Data: []byte("true")},
		"cka/beta/solution/README.md": {Data: []byte("x")},
	}
	e := &env{run: &execx.Fake{}, be: fake.New()}
	e.run.Respond([]string{"kubectl", "config", "view"}, execx.Result{Stdout: []byte("merged")}, nil)
	e.app = &App{Home: t.TempDir(), Version: "test", Layers: layersOf(m), Backend: e.be, Runner: e.run, Binary: "/bin/ks"}
	return e
}

func (e *env) exec(stdin string, args ...string) (code int, out, errOut string) {
	var o, er bytes.Buffer
	code = Execute(context.Background(), e.app, args, &o, &er, strings.NewReader(stdin))
	return code, o.String(), er.String()
}

func TestListAndShow(t *testing.T) {
	e := newEnv(t)
	code, out, _ := e.exec("", "list")
	if code != 0 || !strings.Contains(out, "cka/demo") || strings.Contains(out, "cka/beta") {
		t.Fatalf("list hides experimental: %d\n%s", code, out)
	}
	_, out, _ = e.exec("", "list", "-a")
	if !strings.Contains(out, "cka/beta") {
		t.Fatalf("list -a:\n%s", out)
	}
	_, out, _ = e.exec("", "list", "--json", "--exam", "cka", "--domain", "troubleshooting")
	var entries []listEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil || len(entries) != 1 || entries[0].Nodes != 1 || entries[0].TimeLimit != "10m00s" {
		t.Fatalf("json: %v %s", err, out)
	}
	_, out, _ = e.exec("", "list", "--json", "--exam", "none")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty list must be []: %q", out)
	}
	code, out, _ = e.exec("", "show", "demo")
	if code != 0 || !strings.Contains(out, "Tags:        a, b") || strings.Contains(out, "Do the task") {
		t.Fatalf("show must not leak the prompt: %d\n%s", code, out)
	}
	if code, _, errOut := e.exec("", "show", "nope"); code != ExitError || !strings.Contains(errOut, "kubestronaut list") {
		t.Fatalf("show unknown: %d %s", code, errOut)
	}
}

func TestValidateStatic(t *testing.T) {
	e := newEnv(t)
	code, out, _ := e.exec("", "validate", "--all")
	if code != 0 || !strings.Contains(out, "ok    cka/demo") {
		t.Fatalf("validate: %d\n%s", code, out)
	}
	if code, _, errOut := e.exec("", "validate"); code != ExitError || !strings.Contains(errOut, "--all") {
		t.Fatalf("no args: %d %s", code, errOut)
	}
	if code, _, _ := e.exec("", "validate", "--all", "demo"); code != ExitError {
		t.Fatalf("both: %d", code)
	}
	delete(e.app.Layers[0].(fstest.MapFS), "cka/demo/solution/solve.sh")
	code, _, errOut := e.exec("", "validate", "demo")
	if code != ExitNotPassed || !strings.Contains(errOut, "solution script") {
		t.Fatalf("invalid scenario: %d %s", code, errOut)
	}
}

func TestSessionFlow(t *testing.T) {
	e := newEnv(t)
	for _, cmd := range [][]string{{"status"}, {"prompt"}, {"env"}, {"grade"}, {"hint"}, {"stop"}, {"ssh", "node01", "--", "ls"}} {
		if code, _, errOut := e.exec("", cmd...); code != ExitNoSession || !strings.Contains(errOut, "no active session") {
			t.Fatalf("%v without session: %d %s", cmd, code, errOut)
		}
	}

	code, out, errOut := e.exec("", "start", "demo")
	if code != 0 || !strings.Contains(out, "Do the task.") || !strings.Contains(out, "export KUBECONFIG=") {
		t.Fatalf("start: %d\n%s\n%s", code, out, errOut)
	}
	if code, _, errOut := e.exec("", "start", "demo"); code != ExitError || !strings.Contains(errOut, "--force") {
		t.Fatalf("second start: %d %s", code, errOut)
	}
	_, out, _ = e.exec("", "env")
	if !strings.HasPrefix(out, "export KUBECONFIG=") || !strings.HasSuffix(strings.TrimSpace(out), "kubeconfig") {
		t.Fatalf("env: %q", out)
	}
	_, out, _ = e.exec("", "status")
	for _, want := range []string{"cka/demo (Demo scenario)", "k8s-demo", "controlplane", "of 10m00s suggested"} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q:\n%s", want, out)
		}
	}
	_, out, _ = e.exec("", "prompt")
	if !strings.Contains(out, "Do the task.") {
		t.Fatalf("prompt: %s", out)
	}

	// The pod is absent, so the grade must fail with exit code 1.
	e.run.Respond([]string{"kubectl", "--kubeconfig"}, execx.Result{ExitCode: 1, Stderr: []byte("(NotFound)")}, nil)
	code, out, _ = e.exec("", "grade")
	if code != ExitNotPassed || !strings.Contains(out, "[FAIL] 0/1") || !strings.Contains(out, "not passed (needs 100%)") {
		t.Fatalf("grade: %d\n%s", code, out)
	}
	code, out, _ = e.exec("", "grade", "--json")
	var rep map[string]any
	if code != ExitNotPassed || json.Unmarshal([]byte(out), &rep) != nil || rep["passed"] != false {
		t.Fatalf("grade json: %d %s", code, out)
	}
	// Infra errors on every check give exit 2, not 1.
	e.run.Respond([]string{"kubectl", "--kubeconfig"}, execx.Result{ExitCode: 1, Stderr: []byte("connection refused")}, nil)
	if code, _, errOut := e.exec("", "grade"); code != ExitError || !strings.Contains(errOut, "infrastructure error") {
		t.Fatalf("all errored: %d %s", code, errOut)
	}
	e.run.Respond([]string{"kubectl", "--kubeconfig"}, execx.Result{Stdout: []byte(`{"kind":"Pod","metadata":{"name":"p"}}`)}, nil)
	code, out, _ = e.exec("", "grade", "-v")
	if code != 0 || !strings.Contains(out, "PASSED") {
		t.Fatalf("passing grade: %d\n%s", code, out)
	}

	_, out, _ = e.exec("", "hint")
	if out != "Hint 1/2: first\n" {
		t.Fatalf("hint: %q", out)
	}
	_, out, _ = e.exec("", "hint", "--all")
	if out != "Hint 1/2: first\nHint 2/2: second\n" {
		t.Fatalf("hint --all: %q", out)
	}
	if code, _, errOut := e.exec("n\n", "solution"); code != ExitError || !strings.Contains(errOut, "cancelled") {
		t.Fatalf("declined solution: %d %s", code, errOut)
	}
	if _, out, _ = e.exec("y\n", "solution"); out != "The answer.\n" {
		t.Fatalf("solution: %q", out)
	}
	if _, out, _ = e.exec("", "solution", "--yes"); out != "The answer.\n" {
		t.Fatalf("solution --yes: %q", out)
	}

	e.be.ExecFunc = func(_, _ string, argv []string) (execx.Result, error) {
		return execx.Result{Stdout: []byte("ran " + strings.Join(argv, " ")), ExitCode: 5}, nil
	}
	code, out, _ = e.exec("", "ssh", "node01", "--", "ls", "-l")
	if code != 5 || out != "ran ls -l" {
		t.Fatalf("ssh exit code must pass through: %d %q", code, out)
	}
	if code, _, _ := e.exec("", "ssh", "controlplane"); code != 0 || len(e.be.Shells) != 1 {
		t.Fatalf("ssh shell: %d %v", code, e.be.Shells)
	}

	if code, _, errOut := e.exec("", "reset"); code != ExitError || !strings.Contains(errOut, "cancelled") {
		t.Fatalf("reset without confirmation: %d %s", code, errOut)
	}
	if code, _, _ := e.exec("", "reset", "--yes"); code != 0 {
		t.Fatalf("reset: %d", code)
	}
	if code, out, _ := e.exec("", "stop"); code != 0 || !strings.Contains(out, "Deleting cluster ks-cka-demo") {
		t.Fatalf("stop: %d %s", code, out)
	}
	if code, _, _ := e.exec("", "status"); code != ExitNoSession {
		t.Fatalf("status after stop: %d", code)
	}
}

func TestDoctorAndVersion(t *testing.T) {
	e := newEnv(t)
	e.run.Respond([]string{"docker", "version"}, execx.Result{Stdout: []byte("29.8.2\n")}, nil)
	e.run.Respond([]string{"docker", "info"}, execx.Result{Stdout: []byte("8000000000\n")}, nil)
	e.run.Respond([]string{"kind", "version"}, execx.Result{Stdout: []byte("kind v0.33.0\n")}, nil)
	e.run.Respond([]string{"kubectl", "version"}, execx.Result{Stdout: []byte("Client Version: v1.37.1\n")}, nil)
	code, out, _ := e.exec("", "doctor")
	if code != 0 || !strings.Contains(out, "All good.") || !strings.Contains(out, "7.5 GiB") {
		t.Fatalf("doctor: %d\n%s", code, out)
	}
	e.be.Clusters["ks-orphan"] = fake.Nodes("ks-orphan", 0)
	code, out, _ = e.exec("", "doctor")
	if code != ExitError || !strings.Contains(out, "orphan cluster: ks-orphan") {
		t.Fatalf("doctor orphan: %d\n%s", code, out)
	}
	code, out, _ = e.exec("", "doctor", "--fix")
	if code != 0 || len(e.be.Clusters) != 0 {
		t.Fatalf("doctor --fix: %d\n%s", code, out)
	}
	e.run.Respond([]string{"docker", "info"}, execx.Result{Stdout: []byte("2000000000\n")}, nil)
	e.run.Respond([]string{"kind", "version"}, execx.Result{ExitCode: 127, Stderr: []byte("not found")}, nil)
	if code, out, _ := e.exec("", "doctor"); code != ExitError || !strings.Contains(out, "[FAIL] docker memory") || !strings.Contains(out, "[FAIL] kind") {
		t.Fatalf("doctor failures: %d\n%s", code, out)
	}
	_, out, _ = e.exec("", "version")
	if !strings.Contains(out, "kubestronaut test") || !strings.Contains(out, "docker:  29.8.2") || !strings.Contains(out, "kind:    not available") {
		t.Fatalf("version:\n%s", out)
	}
}

func TestScenariosDirOverrideFlag(t *testing.T) {
	e := newEnv(t)
	e.app.Layers = nil // use the real layering: embedded packs plus --scenarios-dir
	dir := t.TempDir()
	code, out, _ := e.exec("", "--scenarios-dir", dir, "--home", e.app.Home, "list")
	if code != 0 {
		t.Fatalf("list: %d %s", code, out)
	}
}

func TestExitCodeError(t *testing.T) {
	if (&ExitCodeError{Code: 3}).Error() != "exit status 3" {
		t.Fatal("message without cause")
	}
	cause := context.Canceled
	if e := (&ExitCodeError{Code: 2, Err: cause}); e.Error() != cause.Error() || !errors.Is(e, cause) {
		t.Fatal("cause")
	}
}
