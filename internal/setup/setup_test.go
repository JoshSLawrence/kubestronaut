package setup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

func scen(steps ...scenario.SetupStep) *scenario.Scenario {
	s := &scenario.Scenario{ID: "cka/x"}
	for i := range steps {
		if steps[i].Timeout == 0 {
			steps[i].Timeout = scenario.Duration(time.Minute)
		}
	}
	s.Spec.Setup = steps
	return s
}

func TestStepsRunInOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "m"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"m/b.yaml": "kind: B\n", "m/a.yaml": "kind: A\n", "m/skip.txt": "no", "one.yaml": "kind: One\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	k := &kube.Fake{Objects: []map[string]any{{"kind": "Pod", "metadata": map[string]any{"name": "p", "namespace": "default"}}}}
	ex := &execx.Fake{}
	var log bytes.Buffer
	r := &Runner{Kube: k, Exec: ex, Log: &log, Env: []string{"A=1"}, Dir: dir}
	err := r.Run(context.Background(), scen(
		scenario.SetupStep{Script: "setup/s.sh"},
		scenario.SetupStep{Manifest: "m"},
		scenario.SetupStep{Manifest: "one.yaml"},
		scenario.SetupStep{Wait: &scenario.Wait{Ref: kube.Ref{Kind: "pod", Name: "p"}, For: "condition=Ready"}},
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := ex.Calls[0]; got.Name != "bash" || got.Args[0] != "setup/s.sh" || got.Dir != dir || got.Env[0] != "A=1" {
		t.Fatalf("script call: %+v", got)
	}
	var applied []string
	for _, a := range k.Applied {
		applied = append(applied, string(a))
	}
	if strings.Join(applied, ",") != "kind: A,kind: B,kind: One" {
		t.Fatalf("applied: %v", applied)
	}
	if len(k.Waited) != 1 || k.Waited[0].Name != "p" {
		t.Fatalf("waited: %+v", k.Waited)
	}
	if !strings.Contains(log.String(), "step 1/4: script setup/s.sh") {
		t.Fatalf("log:\n%s", log.String())
	}
}

func TestScriptFailures(t *testing.T) {
	ex := &execx.Fake{}
	r := &Runner{Kube: &kube.Fake{}, Exec: ex, Dir: t.TempDir()}
	ex.Respond([]string{"bash"}, execx.Result{ExitCode: 3, Stderr: []byte("a\nb\nc\nd\ne\nf\ng\n")}, nil)
	err := r.Run(context.Background(), scen(scenario.SetupStep{Script: "s.sh"}))
	if err == nil || !strings.Contains(err.Error(), "exited 3") || strings.Contains(err.Error(), "a\nb") || !strings.Contains(err.Error(), "g") {
		t.Fatalf("want the output tail only: %v", err)
	}
	ex.Respond([]string{"bash"}, execx.Result{}, errors.New("no bash"))
	if err := r.Run(context.Background(), scen(scenario.SetupStep{Script: "s.sh"})); err == nil {
		t.Fatal("run error")
	}
}

type blockingRunner struct{ execx.Fake }

func (*blockingRunner) Run(ctx context.Context, _ execx.Cmd) (execx.Result, error) {
	<-ctx.Done()
	return execx.Result{}, ctx.Err()
}

func TestScriptTimeout(t *testing.T) {
	r := &Runner{Kube: &kube.Fake{}, Exec: &blockingRunner{}, Dir: t.TempDir()}
	s := scen(scenario.SetupStep{Script: "s.sh", Timeout: scenario.Duration(20 * time.Millisecond)})
	err := r.Run(context.Background(), s)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err: %v", err)
	}
}

func TestManifestErrors(t *testing.T) {
	r := &Runner{Kube: &kube.Fake{}, Exec: &execx.Fake{}, Dir: t.TempDir()}
	if err := r.Run(context.Background(), scen(scenario.SetupStep{Manifest: "missing.yaml"})); err == nil {
		t.Fatal("missing manifest")
	}
}

func TestWaitStepPollsUntilObjectAppears(t *testing.T) {
	old := pollInterval
	pollInterval = 5 * time.Millisecond
	defer func() { pollInterval = old }()

	k := &kube.Fake{}
	go func() {
		time.Sleep(30 * time.Millisecond)
		k.Add(map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "d", "namespace": "default"}})
	}()
	r := &Runner{Kube: k, Exec: &execx.Fake{}, Dir: t.TempDir()}
	w := scenario.SetupStep{Wait: &scenario.Wait{Ref: kube.Ref{Kind: "deployment", Name: "d"}, For: "condition=Available"}}
	if err := r.Run(context.Background(), scen(w)); err != nil {
		t.Fatal(err)
	}
	if len(k.Waited) != 1 {
		t.Fatal("kubectl wait must run after the object exists")
	}
}

func TestWaitStepErrors(t *testing.T) {
	old := pollInterval
	pollInterval = 5 * time.Millisecond
	defer func() { pollInterval = old }()
	ref := kube.Ref{Kind: "deployment", Name: "ghost"}
	r := &Runner{Kube: &kube.Fake{}, Exec: &execx.Fake{}, Dir: t.TempDir()}
	short := scenario.Duration(40 * time.Millisecond)
	err := r.Run(context.Background(), scen(scenario.SetupStep{Wait: &scenario.Wait{Ref: ref, For: "create"}, Timeout: short}))
	if err == nil || !strings.Contains(err.Error(), "did not appear") {
		t.Fatalf("err: %v", err)
	}
	// "delete" must not wait for the object to exist.
	k := &kube.Fake{WaitErr: errors.New("timed out waiting")}
	r.Kube = k
	err = r.Run(context.Background(), scen(scenario.SetupStep{Wait: &scenario.Wait{Ref: ref, For: "delete"}, Timeout: short}))
	if err == nil || !strings.Contains(err.Error(), "timed out waiting") || len(k.GetRefs) != 0 {
		t.Fatalf("delete wait: %v gets=%d", err, len(k.GetRefs))
	}
}

func TestScriptEnv(t *testing.T) {
	env := ScriptEnv(EnvParams{
		AdminKubeconfig: "/a", Binary: "/bin/ks", Home: "/h", Cluster: "ks-x", Context: "ctx",
		ScenarioDir: "/s", Lib: "/lib.sh",
		Nodes: []backend.Node{{Name: "ks-x-control-plane", Alias: "controlplane"}, {Name: "ks-x-worker", Alias: "node01"}},
	})
	got := strings.Join(env, "\n")
	for _, want := range []string{
		"KUBECONFIG=/a", "KS_BIN=/bin/ks", "KUBESTRONAUT_HOME=/h", "KS_CLUSTER=ks-x", "KS_CONTEXT=ctx",
		"KS_SCENARIO_DIR=/s", "KS_LIB=/lib.sh", "KS_NODE_CONTROLPLANE=ks-x-control-plane", "KS_NODE_NODE01=ks-x-worker",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
}

func TestWriteLibAndHelpers(t *testing.T) {
	dir := t.TempDir()
	lib, err := WriteLib(filepath.Join(dir, "sess"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	script := `source "$KS_LIB"
ks_node_name node01
ks_node_name controlplane
ks_log hello
ks_node_name node09 && echo SHOULD-NOT-PRINT
exit 0`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "KS_LIB="+lib, "KS_NODE_NODE01=w1", "KS_NODE_CONTROLPLANE=cp")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "w1\ncp\n" {
		t.Fatalf("stdout: %q", out.String())
	}
	if !strings.Contains(errb.String(), "[ks] hello") || !strings.Contains(errb.String(), "unknown node alias 'node09'") {
		t.Fatalf("stderr: %q", errb.String())
	}
	if _, err := WriteLib(filepath.Join(lib, "nested")); err == nil {
		t.Fatal("writing below a file must fail")
	}
}

func TestKsWaitNodeRejectsBadState(t *testing.T) {
	dir := t.TempDir()
	lib, err := WriteLib(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	cmd := exec.Command("bash", "-c", `source "$KS_LIB"; ks_wait_node node01 Bogus 5`)
	cmd.Env = append(os.Environ(), "KS_LIB="+lib, "KS_NODE_NODE01=w1", "PATH=/usr/bin:/bin")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err == nil || !strings.Contains(errb.String(), "Ready or NotReady") {
		t.Fatalf("err %v stderr %q", err, errb.String())
	}
}
