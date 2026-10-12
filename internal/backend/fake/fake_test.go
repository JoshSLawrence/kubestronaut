package fake

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

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
