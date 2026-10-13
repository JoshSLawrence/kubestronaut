package fake

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func TestFakeHANodes(t *testing.T) {
	f := New()
	if err := f.Create(context.Background(), backend.ClusterSpec{Name: "ks-a", ControlPlanes: 3, Workers: 1}); err != nil {
		t.Fatal(err)
	}
	nodes, _ := f.Nodes(context.Background(), "ks-a")
	var got []string
	for _, n := range nodes {
		got = append(got, n.Alias+"="+n.Name)
	}
	want := "controlplane=ks-a-control-plane,controlplane02=ks-a-control-plane2,controlplane03=ks-a-control-plane3,node01=ks-a-worker"
	if strings.Join(got, ",") != want {
		t.Fatalf("nodes: %v", got)
	}
	// An unset ControlPlanes still means one.
	if err := f.Create(context.Background(), backend.ClusterSpec{Name: "ks-b"}); err != nil {
		t.Fatal(err)
	}
	if nodes, _ := f.Nodes(context.Background(), "ks-b"); len(nodes) != 1 {
		t.Fatalf("nodes: %+v", nodes)
	}
}

func TestFakeLifecycle(t *testing.T) {
	f := New()
	ctx := context.Background()
	if err := f.Create(ctx, backend.ClusterSpec{Name: "ks-a", Workers: 2}); err != nil {
		t.Fatal(err)
	}
	nodes, err := f.Nodes(ctx, "ks-a")
	if err != nil || len(nodes) != 3 || nodes[2].Alias != "node02" || nodes[2].Name != "ks-a-worker2" {
		t.Fatalf("nodes: %+v %v", nodes, err)
	}
	if l, _ := f.List(ctx); len(l) != 1 || l[0] != "ks-a" {
		t.Fatalf("list: %v", l)
	}
	f.ExecFunc = func(_, _ string, argv []string) (execx.Result, error) {
		return execx.Result{Stdout: []byte(strings.Join(argv, " "))}, nil
	}
	res, _ := f.NodeExec(ctx, "ks-a", "node01", strings.NewReader("in"), "ls", "-l")
	if string(res.Stdout) != "ls -l" || f.Execs[0].Stdin != "in" {
		t.Fatalf("exec: %+v %+v", res, f.Execs)
	}
	if err := f.NodeShell(ctx, "ks-a", "node01"); err != nil || f.Shells[0] != "ks-a/node01" {
		t.Fatal("shell")
	}
	if err := f.LoadImages(ctx, "ks-a", []string{"i"}); err != nil || f.Loaded["ks-a"][0] != "i" {
		t.Fatal("load")
	}
	if err := f.Delete(ctx, "ks-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Nodes(ctx, "ks-a"); err == nil {
		t.Fatal("deleted cluster")
	}
	f.CreateErr, f.DeleteErr = errors.New("c"), errors.New("d")
	if f.Create(ctx, backend.ClusterSpec{Name: "ks-b"}) == nil || f.Delete(ctx, "ks-b") == nil {
		t.Fatal("errors")
	}
	if f.Name() != "fake" {
		t.Fatal("name")
	}
}
