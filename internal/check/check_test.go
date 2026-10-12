package check

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/JoshSLawrence/kubestronaut/internal/backend/fake"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

// spec builds a CheckSpec from YAML the way a scenario file would.
func spec(t *testing.T, y string) scenario.CheckSpec {
	t.Helper()
	j, err := yaml.YAMLToJSON([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	var s scenario.CheckSpec
	if err := json.Unmarshal(j, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func build(t *testing.T, y string) Check {
	t.Helper()
	c, err := Build(spec(t, y))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return c
}

func obj(kind, ns, name string, extra map[string]any) map[string]any {
	meta := map[string]any{"name": name}
	if ns != "" {
		meta["namespace"] = ns
	}
	o := map[string]any{"kind": kind, "metadata": meta}
	for k, v := range extra {
		o[k] = v
	}
	return o
}

func cond(typ, status string) map[string]any {
	return map[string]any{"conditions": []any{map[string]any{"type": typ, "status": status}}}
}

func run(t *testing.T, c Check, env *Env) (Result, error) {
	t.Helper()
	return c.Run(context.Background(), env)
}

func TestBuildErrors(t *testing.T) {
	cases := map[string]string{
		"unknown type":         "id: a\ntype: nope",
		"unknown field":        "id: a\ntype: resourceExists\nref: {kind: pod}\nbogus: 1",
		"missing kind":         "id: a\ntype: resourceExists\nref: {name: x}",
		"count no matcher":     "id: a\ntype: count\nref: {kind: pod}",
		"count eq and gte":     "id: a\ntype: count\nref: {kind: pod}\neq: 1\ngte: 1",
		"jsonPath no matcher":  "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'",
		"jsonPath two":         "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nequals: x\nempty: true",
		"jsonPath no path":     "id: a\ntype: jsonPath\nref: {kind: pod}\nequals: x",
		"jsonPath bad path":    "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a['\nequals: x",
		"jsonPath bad regex":   "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nmatches: '('",
		"jsonPath bad match":   "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nequals: x\nmatch: some",
		"jsonPath empty false": "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nempty: false",
		"jsonPath nonEmpty f":  "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nnonEmpty: false",
		"jsonPath oneOf":       "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\noneOf: []",
		"jsonPath minItems":    "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nequals: x\nminItems: -1",
		"jsonPath bad scalar":  "id: a\ntype: jsonPath\nref: {kind: pod}\npath: '{.a}'\nequals: [1]",
		"ready bad kind":       "id: a\ntype: ready\nref: {kind: service}",
		"ready negative":       "id: a\ntype: ready\nref: {kind: pod}\nminReady: -1",
		"nodeExec no node":     "id: a\ntype: nodeExec\ncommand: ['true']",
		"nodeExec no command":  "id: a\ntype: nodeExec\nnode: node01",
		"nodeExec both":        "id: a\ntype: nodeExec\nnode: node01\ncommand: ['true']\nshell: 'true'",
		"nodeExec bad regex":   "id: a\ntype: nodeExec\nnode: node01\ncommand: ['true']\nstdoutMatches: '('",
		"podExec no name":      "id: a\ntype: podExec\nref: {kind: pod}\ncommand: ['true']",
		"podExec bad":          "id: a\ntype: podExec\nref: {kind: pod, name: p}",
		"podExec no kind":      "id: a\ntype: podExec\nref: {name: p}\ncommand: ['true']",
		"conn bad to":          "id: a\ntype: connectivity\nfrom: {kind: pod, name: p}\nto: nohostport\nexpect: allowed",
		"conn bad expect":      "id: a\ntype: connectivity\nfrom: {kind: pod, name: p}\nto: h:80\nexpect: maybe",
		"conn bad probe":       "id: a\ntype: connectivity\nfrom: {kind: pod, name: p}\nto: h:80\nexpect: allowed\nprobe: curl",
		"conn no from name":    "id: a\ntype: connectivity\nfrom: {kind: pod}\nto: h:80\nexpect: allowed",
		"conn no from":         "id: a\ntype: connectivity\nto: h:80\nexpect: allowed",
		"canI missing":         "id: a\ntype: canI\nverb: get",
		"script no path":       "id: a\ntype: script",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Build(spec(t, y)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestBuildBadRaw(t *testing.T) {
	s := scenario.CheckSpec{Raw: json.RawMessage(`[1]`)}
	s.Type = "canI"
	if _, err := Build(s); err == nil {
		t.Fatal("want error for non-object raw")
	}
}

func TestTypesRegistered(t *testing.T) {
	want := []string{"canI", "connectivity", "count", "jsonPath", "nodeExec", "podExec", "ready", "resourceExists", "script"}
	got := Types()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("types: %v", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration must panic")
		}
	}()
	Register("canI", nil)
}

func TestResourceExistsAndCount(t *testing.T) {
	k := &kube.Fake{Objects: []map[string]any{obj("Pod", "ns", "a", nil), obj("Pod", "ns", "b", nil)}}
	env := &Env{Kube: k}
	cases := []struct {
		y    string
		want bool
	}{
		{"type: resourceExists\nref: {kind: pod, name: a, namespace: ns}", true},
		{"type: resourceExists\nref: {kind: pod, name: z, namespace: ns}", false},
		{"type: resourceExists\nabsent: true\nref: {kind: pod, name: z, namespace: ns}", true},
		{"type: resourceExists\nabsent: true\nref: {kind: pod, name: a, namespace: ns}", false},
		{"type: count\nref: {kind: pod, namespace: ns}\neq: 2", true},
		{"type: count\nref: {kind: pod, namespace: ns}\neq: 3", false},
		{"type: count\nref: {kind: pod, namespace: ns}\ngte: 2\nlte: 2", true},
		{"type: count\nref: {kind: pod, namespace: ns}\ngte: 3", false},
		{"type: count\nref: {kind: pod, namespace: ns}\nlte: 1", false},
	}
	for _, c := range cases {
		res, err := run(t, build(t, "id: x\n"+c.y), env)
		if err != nil || res.Passed != c.want {
			t.Errorf("%q: passed=%v msg=%q err=%v", c.y, res.Passed, res.Message, err)
		}
		if !res.Passed && res.Message == "" {
			t.Errorf("%q: failure needs a message", c.y)
		}
	}
	k.GetErr = errors.New("api down")
	for _, y := range []string{"type: resourceExists\nref: {kind: pod}", "type: count\nref: {kind: pod}\neq: 1"} {
		if _, err := run(t, build(t, "id: x\n"+y), env); err == nil {
			t.Errorf("%q: want infra error", y)
		}
	}
}

func TestJSONPath(t *testing.T) {
	pod := func(name, node string, replicas any) map[string]any {
		return obj("Pod", "ns", name, map[string]any{
			"spec": map[string]any{"nodeName": node, "replicas": replicas, "containers": []any{
				map[string]any{"image": "a:1"}, map[string]any{"image": "b:2"}}},
			"metadata": map[string]any{"name": name, "namespace": "ns", "labels": map[string]any{"app": "x"}},
		})
	}
	k := &kube.Fake{Objects: []map[string]any{pod("p1", "n1", float64(3)), pod("p2", "n2", float64(5))}}
	env := &Env{Kube: k}
	const head = "id: x\ntype: jsonPath\nref: {kind: pod, namespace: ns, selector: app=x}\n"
	cases := []struct {
		name string
		y    string
		want bool
	}{
		{"equals all fails", "path: '{.spec.nodeName}'\nequals: n1", false},
		{"equals any", "path: '{.spec.nodeName}'\nequals: n1\nmatch: any", true},
		{"equals any none", "path: '{.spec.nodeName}'\nequals: n9\nmatch: any", false},
		{"bare path", "path: .spec.nodeName\nnotEquals: nX", true},
		{"notEquals fails", "path: .spec.nodeName\nnotEquals: n1", false},
		{"matches", "path: .spec.nodeName\nmatches: '^n[12]$'", true},
		{"matches fails", "path: .spec.nodeName\nmatches: '^z'", false},
		{"contains", "path: '{.spec.containers[*].image}'\ncontains: b:2", true},
		{"oneOf", "path: .spec.nodeName\noneOf: [n1, n2]", true},
		{"oneOf fails", "path: .spec.nodeName\noneOf: [n3]", false},
		{"gte", "path: .spec.replicas\ngte: 3", true},
		{"gte fails", "path: .spec.replicas\ngte: 4", false},
		{"lte", "path: .spec.replicas\nlte: 5", true},
		{"range", "path: .spec.replicas\ngte: 3\nlte: 5", true},
		{"not numeric", "path: .spec.nodeName\ngte: 1", false},
		{"numeric equals", "path: .spec.replicas\nequals: 3\nmatch: any", true},
		{"missing key empty", "path: .spec.nothing\nempty: true", true},
		{"missing key not empty", "path: .spec.nothing\nnonEmpty: true", false},
		{"missing key equals", "path: .spec.nothing\nequals: x", false},
		{"nonEmpty", "path: .spec.nodeName\nnonEmpty: true", true},
		{"empty fails", "path: .spec.nodeName\nempty: true", false},
		{"min items", "path: .spec.nodeName\nnonEmpty: true\nminItems: 3", false},
		{"bool equals", "path: .spec.nothing\nequals: false", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := run(t, build(t, head+c.y), env)
			if err != nil || res.Passed != c.want {
				t.Fatalf("passed=%v msg=%q err=%v", res.Passed, res.Message, err)
			}
		})
	}
	none := build(t, "id: x\ntype: jsonPath\nref: {kind: pod, name: ghost, namespace: ns}\npath: .a\nempty: true")
	if res, _ := run(t, none, env); res.Passed {
		t.Fatal("no objects must fail by default")
	}
	k.GetErr = errors.New("down")
	if _, err := run(t, none, env); err == nil {
		t.Fatal("want infra error")
	}
}

func TestEvalPathEdge(t *testing.T) {
	if got := evalPath("{.a[", nil); got != "" {
		t.Fatalf("bad path: %q", got)
	}
	if got := evalPath("{.a[5]}", map[string]any{"a": []any{"x"}}); got != "" {
		t.Fatalf("out of range: %q", got)
	}
}

func TestReady(t *testing.T) {
	deployment := func(name string, replicas, readyN, updated, gen, observed float64) map[string]any {
		return obj("Deployment", "ns", name, map[string]any{
			"metadata": map[string]any{"name": name, "namespace": "ns", "generation": gen},
			"spec":     map[string]any{"replicas": replicas},
			"status":   map[string]any{"readyReplicas": readyN, "updatedReplicas": updated, "observedGeneration": observed},
		})
	}
	terminating := obj("Pod", "ns", "old", map[string]any{"status": cond("Ready", "False")})
	terminating["metadata"].(map[string]any)["deletionTimestamp"] = "2026-01-01T00:00:00Z"
	terminating["metadata"].(map[string]any)["labels"] = map[string]any{"app": "w"}
	objs := []map[string]any{
		obj("Pod", "ns", "ready", map[string]any{"status": cond("Ready", "True")}),
		obj("Pod", "ns", "unready", map[string]any{"status": cond("Ready", "False")}),
		obj("Pod", "ns", "nocond", nil),
		terminating,
		obj("Node", "", "n1", map[string]any{"status": cond("Ready", "True")}),
		obj("Node", "", "n2", map[string]any{"status": cond("Ready", "False")}),
		deployment("ok", 2, 2, 2, 1, 1),
		deployment("stuck", 2, 2, 1, 2, 2),
		deployment("stale", 1, 1, 1, 2, 1),
		deployment("down", 2, 1, 2, 1, 1),
		obj("StatefulSet", "ns", "sts", map[string]any{"spec": map[string]any{"replicas": float64(2)}, "status": map[string]any{"readyReplicas": float64(2)}}),
		obj("StatefulSet", "ns", "sts-down", map[string]any{"status": map[string]any{"readyReplicas": float64(0)}}),
		obj("DaemonSet", "ns", "ds", map[string]any{"status": map[string]any{"desiredNumberScheduled": float64(2), "numberReady": float64(2)}}),
		obj("DaemonSet", "ns", "ds-none", map[string]any{"status": map[string]any{}}),
		obj("PersistentVolumeClaim", "ns", "bound", map[string]any{"status": map[string]any{"phase": "Bound"}}),
		obj("PersistentVolumeClaim", "ns", "pending", map[string]any{"status": map[string]any{"phase": "Pending"}}),
		obj("Job", "ns", "done", map[string]any{"status": cond("Complete", "True")}),
		obj("Job", "ns", "running", nil),
	}
	env := &Env{Kube: &kube.Fake{Objects: objs}}
	cases := []struct {
		ref      string
		minReady int
		want     bool
	}{
		{"{kind: pod, name: ready, namespace: ns}", 0, true},
		{"{kind: pod, name: unready, namespace: ns}", 0, false},
		{"{kind: pod, name: nocond, namespace: ns}", 0, false},
		{"{kind: pod, namespace: ns}", 0, false},
		{"{kind: pod, namespace: ns}", 1, true},
		{"{kind: pod, namespace: ns}", 2, false},
		{"{kind: pod, name: old, namespace: ns}", 0, false},
		{"{kind: pod, name: ghost, namespace: ns}", 0, false},
		{"{kind: node, name: n1}", 0, true},
		{"{kind: nodes, name: n2}", 0, false},
		{"{kind: deployment, name: ok, namespace: ns}", 0, true},
		{"{kind: deploy, name: ok, namespace: ns}", 2, true},
		{"{kind: deployment, name: ok, namespace: ns}", 3, false},
		{"{kind: deployment, name: stuck, namespace: ns}", 0, false},
		{"{kind: deployment, name: stale, namespace: ns}", 0, false},
		{"{kind: deployment, name: down, namespace: ns}", 0, false},
		{"{kind: deployment, name: down, namespace: ns}", 1, true},
		{"{kind: statefulset, name: sts, namespace: ns}", 0, true},
		{"{kind: sts, name: sts-down, namespace: ns}", 0, false},
		{"{kind: daemonset, name: ds, namespace: ns}", 0, true},
		{"{kind: ds, name: ds-none, namespace: ns}", 0, false},
		{"{kind: pvc, name: bound, namespace: ns}", 0, true},
		{"{kind: pvc, name: pending, namespace: ns}", 0, false},
		{"{kind: job, name: done, namespace: ns}", 0, true},
		{"{kind: job, name: running, namespace: ns}", 0, false},
	}
	for _, c := range cases {
		y := "id: x\ntype: ready\nref: " + c.ref
		if c.minReady > 0 {
			y += "\nminReady: " + strconv.Itoa(c.minReady)
		}
		res, err := run(t, build(t, y), env)
		if err != nil || res.Passed != c.want {
			t.Errorf("%s minReady=%d: passed=%v msg=%q err=%v", c.ref, c.minReady, res.Passed, res.Message, err)
		}
	}
	env.Kube.(*kube.Fake).GetErr = errors.New("down")
	if _, err := run(t, build(t, "id: x\ntype: ready\nref: {kind: pod}"), env); err == nil {
		t.Fatal("want infra error")
	}
}

func TestNodeExec(t *testing.T) {
	b := fake.New()
	env := &Env{Backend: b, Cluster: "ks-x"}
	var res execx.Result
	b.ExecFunc = func(_, _ string, _ []string) (execx.Result, error) { return res, nil }
	cases := []struct {
		y    string
		res  execx.Result
		want bool
	}{
		{"command: [systemctl, is-enabled, kubelet]", execx.Result{Stdout: []byte("enabled\n")}, true},
		{"command: [x]", execx.Result{ExitCode: 1}, false},
		{"command: [x]\nexitCode: 1", execx.Result{ExitCode: 1}, true},
		{"command: [x]\nstdoutContains: enabled", execx.Result{Stdout: []byte("enabled")}, true},
		{"command: [x]\nstdoutContains: enabled", execx.Result{Stdout: []byte("disabled?")}, false},
		{"command: [x]\nstdoutContains: 'yes'", execx.Result{Stdout: []byte("no")}, false},
		{"shell: echo hi", execx.Result{Stdout: []byte("hi")}, true},
		{"command: [x]\nstdoutMatches: '^v1\\.3[0-9]'", execx.Result{Stdout: []byte("v1.36.4")}, true},
		{"command: [x]\nstdoutMatches: '^v2'", execx.Result{Stdout: []byte("v1.36.4")}, false},
	}
	for _, c := range cases {
		res = c.res
		got, err := run(t, build(t, "id: x\ntype: nodeExec\nnode: node01\n"+c.y), env)
		if err != nil || got.Passed != c.want {
			t.Errorf("%q: passed=%v msg=%q err=%v", c.y, got.Passed, got.Message, err)
		}
	}
	last := b.Execs[len(b.Execs)-1]
	if last.Alias != "node01" || last.Cluster != "ks-x" {
		t.Fatalf("exec target: %+v", last)
	}
	shellCall := b.Execs[6]
	if strings.Join(shellCall.Argv, " ") != "bash -c echo hi" {
		t.Fatalf("shell argv: %v", shellCall.Argv)
	}
	b.ExecFunc = func(_, _ string, _ []string) (execx.Result, error) { return execx.Result{}, errors.New("no node") }
	if _, err := run(t, build(t, "id: x\ntype: nodeExec\nnode: node01\ncommand: [x]"), env); err == nil {
		t.Fatal("want infra error")
	}
}

func TestPodExec(t *testing.T) {
	k := &kube.Fake{Objects: []map[string]any{obj("Pod", "ns", "p", nil)}}
	env := &Env{Kube: k}
	var res execx.Result
	var gotPod string
	k.ExecFunc = func(ns, pod, container string, _ []string) (execx.Result, error) {
		gotPod = ns + "/" + pod + "/" + container
		return res, nil
	}
	run1 := func(y string) Result {
		r, err := run(t, build(t, "id: x\ntype: podExec\n"+y), env)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := run1("ref: {kind: pod, name: p, namespace: ns}\ncontainer: c\ncommand: ['true']"); !r.Passed || gotPod != "ns/p/c" {
		t.Fatalf("by name: %+v %s", r, gotPod)
	}
	if r := run1("ref: {kind: pod, namespace: ns, selector: x}\ncommand: ['true']"); r.Passed || !strings.Contains(r.Message, "not found") {
		// the fake selector excludes the pod, which has no labels
		t.Fatalf("selector without match: %+v", r)
	}
	k.Objects[0]["metadata"].(map[string]any)["labels"] = map[string]any{"x": "1"}
	if r := run1("ref: {kind: pod, namespace: ns, selector: x}\ncommand: ['true']"); !r.Passed || gotPod != "ns/p/" {
		t.Fatalf("by selector: %+v %s", r, gotPod)
	}
	res = execx.Result{ExitCode: 1, Stderr: []byte(`Error from server (NotFound): pods "p" not found`)}
	if r := run1("ref: {kind: pod, name: p, namespace: ns}\ncommand: ['true']"); r.Passed {
		t.Fatal("kubectl failure must not pass")
	}
	res = execx.Result{ExitCode: 2}
	if r := run1("ref: {kind: pod, name: p, namespace: ns}\ncommand: ['true']\nexitCode: 2"); !r.Passed {
		t.Fatalf("expected exit code: %+v", r)
	}
	k.ExecFunc = func(string, string, string, []string) (execx.Result, error) {
		return execx.Result{}, errors.New("boom")
	}
	if _, err := run(t, build(t, "id: x\ntype: podExec\nref: {kind: pod, name: p}\ncommand: ['true']"), env); err == nil {
		t.Fatal("want infra error")
	}
	k.GetErr = errors.New("down")
	if _, err := run(t, build(t, "id: x\ntype: podExec\nref: {kind: pod, selector: x}\ncommand: ['true']"), env); err == nil {
		t.Fatal("want infra error from Get")
	}
}

func TestConnectivity(t *testing.T) {
	k := &kube.Fake{Objects: []map[string]any{obj("Pod", "shop", "web", nil)}}
	env := &Env{Kube: k}
	var res execx.Result
	var argv []string
	k.ExecFunc = func(_, _, _ string, a []string) (execx.Result, error) {
		if a[0] == "nslookup" {
			return execx.Result{}, nil
		}
		argv = a
		return res, nil
	}
	const head = "id: x\ntype: connectivity\nfrom: {kind: pod, name: web, namespace: shop}\nto: api.shop.svc:8080\n"
	run1 := func(extra string) (Result, error) { return run(t, build(t, head+extra), env) }

	res = execx.Result{}
	if r, err := run1("expect: allowed"); err != nil || !r.Passed {
		t.Fatalf("allowed: %+v %v", r, err)
	}
	if strings.Join(argv, " ") != "/agnhost connect api.shop.svc:8080 --timeout=3s" {
		t.Fatalf("agnhost argv: %v", argv)
	}
	if r, _ := run1("expect: denied"); r.Passed {
		t.Fatal("connected but denied expected")
	}
	res = execx.Result{ExitCode: 1, Stderr: []byte("TIMEOUT: nope\ncommand terminated with exit code 1")}
	if r, err := run1("expect: denied"); err != nil || !r.Passed {
		t.Fatalf("denied: %+v %v", r, err)
	}
	if r, _ := run1("expect: allowed"); r.Passed || r.Message == "" {
		t.Fatal("blocked but allowed expected")
	}
	res = execx.Result{ExitCode: 1, Stdout: []byte("DNS: lookup failed")}
	if _, err := run1("expect: denied"); err == nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("DNS failure must be an infra error, got %v", err)
	}
	res = execx.Result{ExitCode: 1, Stderr: []byte("REFUSED: connection refused")}
	if r, err := run1("expect: denied"); err != nil || !r.Passed {
		t.Fatalf("refused counts as denied: %+v %v", r, err)
	}
	res = execx.Result{ExitCode: 1, Stderr: []byte("UNKNOWN: weird")}
	if _, err := run1("expect: denied"); err == nil || !strings.Contains(err.Error(), "unexpected probe output") {
		t.Fatalf("unrecognised output must be an infra error: %v", err)
	}
	if r, err := run1("expect: allowed"); err != nil || r.Passed {
		t.Fatalf("unrecognised failure still fails an allowed check: %+v %v", r, err)
	}
	res = execx.Result{ExitCode: 1, Stderr: []byte(`Error from server (BadRequest): pod web does not have a host assigned`)}
	if _, err := run1("expect: denied"); err == nil {
		t.Fatal("kubectl failure must be an infra error")
	}
	res = execx.Result{ExitCode: 1}
	if r, err := run1("expect: denied\nprobe: nc\nconnectTimeout: 5s"); err != nil || !r.Passed {
		t.Fatalf("nc: %+v %v", r, err)
	}
	if strings.Join(argv, " ") != "nc -z -w 5 api.shop.svc 8080" {
		t.Fatalf("nc argv: %v", argv)
	}
	// nc: a name that does not resolve must not read as "denied".
	k.ExecFunc = func(_, _, _ string, a []string) (execx.Result, error) {
		if a[0] == "nslookup" {
			return execx.Result{ExitCode: 1, Stdout: []byte("** server can't find")}, nil
		}
		return execx.Result{ExitCode: 1}, nil
	}
	if _, err := run1("expect: denied\nprobe: nc"); err == nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("nc DNS failure: %v", err)
	}
	k.ExecFunc = func(_, _, _ string, a []string) (execx.Result, error) {
		if a[0] == "nslookup" {
			return execx.Result{ExitCode: 127, Stderr: []byte("nslookup: not found")}, nil
		}
		return execx.Result{ExitCode: 1, Stderr: []byte("nc: bad address 'api.shop.svc'")}, nil
	}
	if _, err := run1("expect: denied\nprobe: nc"); err == nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("nc bad address: %v", err)
	}
	k.ExecFunc = func(_, _, _ string, a []string) (execx.Result, error) {
		if a[0] == "nslookup" {
			return execx.Result{Stderr: []byte(`Error from server (NotFound): pods "web" not found`), ExitCode: 1}, nil
		}
		return execx.Result{ExitCode: 1}, nil
	}
	if _, err := run1("expect: denied\nprobe: nc"); err == nil {
		t.Fatal("kubectl failure during the lookup")
	}
	k.ExecFunc = func(_, _, _ string, a []string) (execx.Result, error) {
		if a[0] == "nslookup" {
			return execx.Result{}, nil
		}
		return execx.Result{ExitCode: 1}, nil
	}
	if r, err := run1("expect: denied\nprobe: nc"); err != nil || !r.Passed {
		t.Fatalf("resolvable and blocked is denied: %+v %v", r, err)
	}
	k.ExecFunc = func(string, string, string, []string) (execx.Result, error) {
		return execx.Result{}, errors.New("boom")
	}
	if _, err := run1("expect: denied\nprobe: nc"); err == nil {
		t.Fatal("exec error during lookup")
	}
	k.Objects = nil
	if _, err := run1("expect: denied"); err == nil {
		t.Fatal("missing source pod must be an infra error")
	}
	k.GetErr = errors.New("down")
	if _, err := run1("expect: denied"); err == nil {
		t.Fatal("Get error")
	}
	k.GetErr = nil
	k.Objects = []map[string]any{obj("Pod", "shop", "web", nil)}
	k.ExecFunc = func(string, string, string, []string) (execx.Result, error) {
		return execx.Result{}, errors.New("boom")
	}
	if _, err := run1("expect: denied"); err == nil {
		t.Fatal("exec error")
	}
}

func TestCanI(t *testing.T) {
	k := &kube.Fake{CanIAnswers: map[string]bool{
		kube.CanIKey("sa", "create", "deployments", "", "dev"): true,
		kube.CanIKey("sa", "get", "secrets", "", "dev"):        false,
		kube.CanIKey("sa", "get", "pods", "log", "dev"):        true,
	}}
	env := &Env{Kube: k}
	cases := []struct {
		y    string
		want bool
	}{
		{"verb: create\nresource: deployments", true},
		{"verb: create\nresource: deployments\nallowed: false", false},
		{"verb: get\nresource: secrets\nallowed: false", true},
		{"verb: get\nresource: secrets", false},
		{"verb: get\nresource: pods\nsubresource: log", true},
	}
	for _, c := range cases {
		res, err := run(t, build(t, "id: x\ntype: canI\nas: sa\nnamespace: dev\n"+c.y), env)
		if err != nil || res.Passed != c.want {
			t.Errorf("%q: %+v %v", c.y, res, err)
		}
		if !res.Passed && res.Message == "" {
			t.Errorf("%q: needs a message", c.y)
		}
	}
	if _, err := run(t, build(t, "id: x\ntype: canI\nas: sa\nverb: list\nresource: nodes"), env); err == nil {
		t.Fatal("want infra error for unanswered call")
	}
}

func TestScript(t *testing.T) {
	r := &execx.Fake{}
	env := &Env{Runner: r, Dir: "/scn", ScriptEnv: []string{"KS_CLUSTER=c"}}
	c := build(t, "id: x\ntype: script\npath: checks/a.sh\nargs: [one]\nenv: {B: '2', A: '1'}")
	res, err := run(t, c, env)
	if err != nil || !res.Passed {
		t.Fatalf("pass: %+v %v", res, err)
	}
	call := r.Calls[0]
	if call.Name != "bash" || strings.Join(call.Args, " ") != "checks/a.sh one" || call.Dir != "/scn" ||
		strings.Join(call.Env, ",") != "KS_CLUSTER=c,A=1,B=2" {
		t.Fatalf("call: %+v", call)
	}
	r.Respond([]string{"bash"}, execx.Result{ExitCode: 1, Stderr: []byte("noise\nwhy it failed\n")}, nil)
	if res, _ := run(t, c, env); res.Passed || res.Message != "why it failed" {
		t.Fatalf("fail: %+v", res)
	}
	r.Respond([]string{"bash"}, execx.Result{ExitCode: 4}, nil)
	if res, _ := run(t, c, env); res.Message != "script exited 4" {
		t.Fatalf("fallback message: %+v", res)
	}
	r.Respond([]string{"bash"}, execx.Result{}, errors.New("no bash"))
	if _, err := run(t, c, env); err == nil {
		t.Fatal("want infra error")
	}
}

type flaky struct {
	failures int32
	calls    atomic.Int32
	block    bool
	err      error
}

func (f *flaky) Run(ctx context.Context, _ *Env) (Result, error) {
	n := f.calls.Add(1)
	if f.block {
		<-ctx.Done()
		return Result{}, nil
	}
	if f.err != nil {
		return Result{}, f.err
	}
	if n > f.failures {
		return pass(), nil
	}
	return fail("not yet"), nil
}

func TestRunWithPolicy(t *testing.T) {
	sp := func(attempts int, timeout time.Duration) scenario.CheckSpec {
		var s scenario.CheckSpec
		s.Retry = scenario.Retry{Attempts: attempts, Interval: scenario.Duration(time.Millisecond)}
		s.Timeout = scenario.Duration(timeout)
		return s
	}
	ctx := context.Background()

	f := &flaky{failures: 2}
	if res, err := RunWithPolicy(ctx, &Env{}, sp(5, time.Second), f); err != nil || !res.Passed || f.calls.Load() != 3 {
		t.Fatalf("retry until pass: %+v %v calls=%d", res, err, f.calls.Load())
	}
	f = &flaky{failures: 10}
	if res, err := RunWithPolicy(ctx, &Env{}, sp(3, time.Second), f); err != nil || res.Passed || f.calls.Load() != 3 {
		t.Fatalf("exhausted: %+v %v calls=%d", res, err, f.calls.Load())
	}
	f = &flaky{block: true}
	start := time.Now()
	if _, err := RunWithPolicy(ctx, &Env{}, sp(1, 30*time.Millisecond), f); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout not enforced")
	}
	f = &flaky{err: errors.New("infra")}
	if _, err := RunWithPolicy(ctx, &Env{}, sp(2, time.Second), f); err == nil || f.calls.Load() != 2 {
		t.Fatalf("infra error retried: %v calls=%d", err, f.calls.Load())
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RunWithPolicy(cctx, &Env{}, sp(3, time.Second), &flaky{failures: 9}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	// Zero values fall back to sane defaults.
	if res, err := RunWithPolicy(ctx, &Env{}, scenario.CheckSpec{}, &flaky{}); err != nil || !res.Passed {
		t.Fatalf("defaults: %+v %v", res, err)
	}
}

func TestSleepHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	sleep(ctx, time.Hour)
	if time.Since(start) > time.Second {
		t.Fatal("sleep must return when the context ends")
	}
}

func TestValidate(t *testing.T) {
	s := &scenario.Scenario{ID: "cka/x"}
	s.Spec.Checks = []scenario.CheckSpec{
		spec(t, "id: ok\ntype: canI\nas: a\nverb: get\nresource: pods"),
		spec(t, "id: bad\ntype: nope"),
	}
	errs := Validate(s)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "cka/x: checks[1] (bad)") {
		t.Fatalf("errs: %v", errs)
	}
}
