package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func newKubectl() (*Kubectl, *execx.Fake) {
	f := &execx.Fake{}
	return &Kubectl{Runner: f, Kubeconfig: "/kc"}, f
}

func TestKubectlGet(t *testing.T) {
	k, f := newKubectl()
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "pod", "-n", "ns", "-l", "a=b"},
		execx.Result{Stdout: []byte(`{"kind":"List","items":[{"kind":"Pod","metadata":{"name":"a"}},{"kind":"Pod","metadata":{"name":"b"}}]}`)}, nil)
	objs, err := k.Get(context.Background(), Ref{Kind: "pod", Namespace: "ns", Selector: "a=b"})
	if err != nil || len(objs) != 2 {
		t.Fatalf("objs %v err %v", objs, err)
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "node", "n1"},
		execx.Result{Stdout: []byte(`{"kind":"Node","metadata":{"name":"n1"}}`)}, nil)
	objs, err = k.Get(context.Background(), Ref{Kind: "node", Name: "n1"})
	if err != nil || len(objs) != 1 {
		t.Fatalf("single: %v %v", objs, err)
	}
	if got := f.Lines()[1]; !strings.HasSuffix(got, "get node n1 -o json") {
		t.Fatalf("command: %s", got)
	}
}

func TestKubectlGetArgsAndErrors(t *testing.T) {
	k, f := newKubectl()
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "pod", "x"},
		execx.Result{ExitCode: 1, Stderr: []byte(`Error from server (NotFound): pods "x" not found`)}, nil)
	objs, err := k.Get(context.Background(), Ref{Kind: "pod", Name: "x"})
	if err != nil || objs != nil {
		t.Fatalf("NotFound must be empty: %v %v", objs, err)
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "widget"},
		execx.Result{ExitCode: 1, Stderr: []byte("the server doesn't have a resource type")}, nil)
	if _, err := k.Get(context.Background(), Ref{Kind: "widget", AllNamespaces: true, FieldSelector: "a=b"}); err == nil {
		t.Fatal("want error")
	}
	if got := f.Lines()[1]; !strings.Contains(got, "-A") || !strings.Contains(got, "--field-selector a=b") {
		t.Fatalf("args: %s", got)
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "junk"}, execx.Result{Stdout: []byte("not json")}, nil)
	if _, err := k.Get(context.Background(), Ref{Kind: "junk"}); err == nil {
		t.Fatal("want decode error")
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "get", "boom"}, execx.Result{}, errors.New("no binary"))
	if _, err := k.Get(context.Background(), Ref{Kind: "boom"}); err == nil {
		t.Fatal("want run error")
	}
}

func TestKubectlApplyExecCanIWait(t *testing.T) {
	k, f := newKubectl()
	ctx := context.Background()
	if err := k.Apply(ctx, []byte("kind: X")); err != nil {
		t.Fatal(err)
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "apply"}, execx.Result{ExitCode: 1, Stderr: []byte("bad")}, nil)
	if err := k.Apply(ctx, nil); err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("apply err: %v", err)
	}

	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "exec"}, execx.Result{ExitCode: 7}, nil)
	res, err := k.Exec(ctx, "ns", "p", "c", []string{"ls", "-l"})
	if err != nil || res.ExitCode != 7 {
		t.Fatalf("exec %v %v", res, err)
	}
	if got := f.Lines()[len(f.Lines())-1]; !strings.HasSuffix(got, "exec p -n ns -c c -- ls -l") {
		t.Fatalf("exec args: %s", got)
	}

	f.Once([]string{"kubectl", "--kubeconfig", "/kc", "auth"}, execx.Result{Stdout: []byte("yes\n")}, nil)
	f.Once([]string{"kubectl", "--kubeconfig", "/kc", "auth"}, execx.Result{Stdout: []byte("no\n"), ExitCode: 1}, nil)
	f.Once([]string{"kubectl", "--kubeconfig", "/kc", "auth"}, execx.Result{Stderr: []byte("forbidden"), ExitCode: 1}, nil)
	if ok, err := k.CanI(ctx, "u", "get", "pods", "log", "ns"); !ok || err != nil {
		t.Fatalf("yes: %v %v", ok, err)
	}
	if ok, err := k.CanI(ctx, "", "get", "pods", "", ""); ok || err != nil {
		t.Fatalf("no: %v %v", ok, err)
	}
	if _, err := k.CanI(ctx, "", "get", "pods", "", ""); err == nil {
		t.Fatal("want error")
	}

	if err := k.Wait(ctx, Ref{Kind: "pod", Name: "p"}, "condition=Ready", 90*time.Second); err != nil {
		t.Fatal(err)
	}
	if got := f.Lines()[len(f.Lines())-1]; !strings.HasSuffix(got, "wait pod p --for=condition=Ready --timeout=90s") {
		t.Fatalf("wait args: %s", got)
	}
	f.Respond([]string{"kubectl", "--kubeconfig", "/kc", "wait"}, execx.Result{ExitCode: 1, Stderr: []byte("timed out")}, nil)
	if err := k.Wait(ctx, Ref{Kind: "pod"}, "delete", time.Second); err == nil {
		t.Fatal("want wait error")
	}
}

func TestFakeGet(t *testing.T) {
	f := &Fake{Objects: []map[string]any{
		{"kind": "Pod", "metadata": map[string]any{"name": "a", "namespace": "ns", "labels": map[string]any{"app": "x", "tier": "web"}}, "spec": map[string]any{"nodeName": "n1"}},
		{"kind": "Pod", "metadata": map[string]any{"name": "b", "namespace": "other"}},
		{"kind": "NetworkPolicy", "metadata": map[string]any{"name": "np", "namespace": "default"}},
		{"kind": "Node", "metadata": map[string]any{"name": "n1"}},
	}}
	ctx := context.Background()
	cases := []struct {
		ref  Ref
		want int
	}{
		{Ref{Kind: "pods", Namespace: "ns"}, 1},
		{Ref{Kind: "pod", AllNamespaces: true}, 2},
		{Ref{Kind: "pod", Namespace: "ns", Selector: "app=x,tier"}, 1},
		{Ref{Kind: "pod", Namespace: "ns", Selector: "app!=x"}, 0},
		{Ref{Kind: "pod", Namespace: "ns", Selector: "nope"}, 0},
		{Ref{Kind: "pod", Namespace: "ns", FieldSelector: "spec.nodeName=n1"}, 1},
		{Ref{Kind: "pod", Namespace: "ns", FieldSelector: "spec.nodeName=n2"}, 0},
		{Ref{Kind: "pod", Name: "b", Namespace: "other"}, 1},
		{Ref{Kind: "networkpolicies"}, 1},
		{Ref{Kind: "node", Name: "n1"}, 1},
	}
	for _, c := range cases {
		got, err := f.Get(ctx, c.ref)
		if err != nil || len(got) != c.want {
			t.Errorf("%+v: got %d err %v, want %d", c.ref, len(got), err, c.want)
		}
	}
	f.GetErr = ErrFake()
	if _, err := f.Get(ctx, Ref{Kind: "pod"}); err == nil {
		t.Fatal("want GetErr")
	}
}

func TestFakeOthers(t *testing.T) {
	f := &Fake{CanIAnswers: map[string]bool{CanIKey("u", "get", "pods", "", "ns"): true}}
	ctx := context.Background()
	if ok, err := f.CanI(ctx, "u", "get", "pods", "", "ns"); !ok || err != nil {
		t.Fatal("want yes")
	}
	if _, err := f.CanI(ctx, "u", "get", "nodes", "", ""); err == nil {
		t.Fatal("want unknown error")
	}
	if err := f.Apply(ctx, []byte("x")); err != nil || len(f.Applied) != 1 {
		t.Fatal("apply")
	}
	if res, err := f.Exec(ctx, "n", "p", "", nil); err != nil || res.ExitCode != 0 {
		t.Fatal("default exec")
	}
	f.ExecFunc = func(_, _, _ string, _ []string) (execx.Result, error) { return execx.Result{ExitCode: 4}, nil }
	if res, _ := f.Exec(ctx, "n", "p", "", nil); res.ExitCode != 4 {
		t.Fatal("exec func")
	}
	f.WaitErr = ErrFake()
	if err := f.Wait(ctx, Ref{Kind: "pod"}, "create", time.Second); err == nil || len(f.Waited) != 1 {
		t.Fatal("wait")
	}
}
