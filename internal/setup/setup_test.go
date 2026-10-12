package setup

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestWaitTimeoutPrecedence(t *testing.T) {
	sec := func(n int) scenario.Duration { return scenario.Duration(time.Duration(n) * time.Second) }
	cases := []struct {
		name string
		st   scenario.SetupStep
		want time.Duration
	}{
		{"wait.timeout wins", scenario.SetupStep{Timeout: sec(5), Wait: &scenario.Wait{Timeout: sec(7)}}, 7 * time.Second},
		{"step timeout next", scenario.SetupStep{Timeout: sec(5), Wait: &scenario.Wait{}}, 5 * time.Second},
		{"default last", scenario.SetupStep{Wait: &scenario.Wait{}}, scenario.DefaultWaitTimeout},
	}
	for _, c := range cases {
		if got := waitTimeout(c.st); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestWaitStepHonoursWaitTimeout(t *testing.T) {
	old := pollInterval
	pollInterval = 5 * time.Millisecond
	defer func() { pollInterval = old }()
	r := &Runner{Kube: &kube.Fake{}, Exec: &execx.Fake{}, Dir: t.TempDir()}
	// The step timeout alone would wait a minute; wait.timeout must cut it.
	st := scenario.SetupStep{Timeout: scenario.Duration(time.Minute),
		Wait: &scenario.Wait{Ref: kube.Ref{Kind: "pod", Name: "ghost"}, For: "create", Timeout: scenario.Duration(40 * time.Millisecond)}}
	start := time.Now()
	if err := r.Run(context.Background(), scen(st)); err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("wait.timeout ignored: err=%v after %s", err, time.Since(start))
	}
}

// bashLib runs script with the helper library sourced. ks_node is replaced by
// a stand-in that runs the command locally, so node helpers work offline.
func bashLib(t *testing.T, scenarioDir, script string) (string, string, error) {
	t.Helper()
	return bashLibEnv(t, scenarioDir, script)
}

func bashLibEnv(t *testing.T, scenarioDir, script string, extra ...string) (string, string, error) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	lib, err := WriteLib(filepath.Join(t.TempDir(), "sess"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `source "$KS_LIB"; ks_node() { shift; "$@"; }; eval "$KS_TEST_SCRIPT"`)
	cmd.Env = append(os.Environ(), "KS_LIB="+lib, "KS_SCENARIO_DIR="+scenarioDir, "KS_TEST_SCRIPT="+script)
	cmd.Env = append(cmd.Env, extra...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	return out.String(), errb.String(), err
}

func TestKsCopyTo(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "sub", "a.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		// An extended attribute makes macOS tar emit a "._a.txt" entry.
		if _, errs, err := bashLib(t, t.TempDir(), "xattr -w com.example.test x '"+file+"'"); err != nil {
			t.Skipf("cannot set an extended attribute: %v %s", err, errs)
		}
	}
	base := t.TempDir()
	dest := filepath.Join(base, "new", "dir")
	stream := filepath.Join(base, "stream.tar")
	// The stand-in node records the archive it is sent and unpacks it with the
	// same flags the real node gets.
	script := `ks_node() { shift; if [ "$1" = tar ]; then tee "$KS_TEST_STREAM" | "$@"; else "$@"; fi; }
ks_copy_to node01 '` + src + `' '` + dest + `'
cat '` + dest + `/sub/a.txt'`
	out, errs, err := bashLibEnv(t, t.TempDir(), script, "KS_TEST_STREAM="+stream)
	if err != nil || out != "hello" {
		t.Fatalf("copy: out=%q err=%v stderr=%s", out, err, errs)
	}
	f, err := os.Open(filepath.Clean(stream))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	var names []string
	for tr := tar.NewReader(f); ; {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	listing := strings.Join(names, "\n")
	if strings.Contains(listing, "._") {
		t.Fatalf("the archive carries AppleDouble entries:\n%s", listing)
	}
	if !strings.Contains(listing, "sub/a.txt") {
		t.Fatalf("the archive lacks the file:\n%s", listing)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	// The source directory is 0700; the destination must not inherit that.
	if info.Mode().Perm()&0o055 != 0o055 {
		t.Fatalf("destination mode %v, want world readable", info.Mode().Perm())
	}
	if _, errs, err = bashLib(t, t.TempDir(), "ks_copy_to node01 /does/not/exist /x"); err == nil || !strings.Contains(errs, "not a directory") {
		t.Fatalf("missing source: err=%v stderr=%q", err, errs)
	}
}

func TestKsAnswers(t *testing.T) {
	sd := t.TempDir()
	if _, errs, err := bashLib(t, sd, `ks_answer_put token s3cret`); err != nil {
		t.Fatalf("put: %v %s", err, errs)
	}
	info, err := os.Stat(filepath.Join(sd, ".answers", "token"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("answer mode %v, want 0600", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(filepath.Clean(filepath.Join(sd, ".answers", "token")))
	if strings.Contains(string(raw), "s3cret") || len(strings.TrimSpace(string(raw))) != 64 {
		t.Fatalf("answer must be a sha256, got %q", raw)
	}
	for script, wantOK := range map[string]bool{
		`ks_answer_check token s3cret`:                                 true,
		`ks_answer_check token s3cret2`:                                false,
		`ks_answer_check token ''`:                                     false,
		`ks_answer_check nokey s3cret`:                                 false,
		`ks_answer_check ../token s3cret`:                              false,
		`ks_answer_put '../escape' x`:                                  false,
		`ks_answer_put '.hidden' x`:                                    false,
		`ks_answer_put '' x`:                                           false,
		`ks_answer_put multi $'a\nb' && ks_answer_check multi $'a\nb'`: true,
	} {
		_, errs, err := bashLib(t, sd, script)
		if (err == nil) != wantOK {
			t.Errorf("%s: err=%v stderr=%q, want ok=%v", script, err, errs, wantOK)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(sd), "escape")); err == nil {
		t.Fatal("a key must not escape the answers directory")
	}
}
