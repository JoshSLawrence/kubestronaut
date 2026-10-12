package kubeconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func writeAdmin(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name+".kubeconfig")
	if err := os.WriteFile(p, []byte("admin of "+name), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	f := &execx.Fake{}
	f.Respond([]string{"kubectl", "config", "view"}, execx.Result{Stdout: []byte("merged")}, nil)
	out := filepath.Join(dir, "home", "kubeconfig")
	err := Generate(context.Background(), f, []Entry{
		{Admin: writeAdmin(t, dir, "a"), Cluster: "ks-a", Context: "k8s-a"},
		{Admin: writeAdmin(t, dir, "b"), Cluster: "ks-b", Context: "k8s-b"},
	}, "k8s-b", out)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Clean(out))
	if string(b) != "merged" {
		t.Fatalf("output: %q", b)
	}
	lines := f.Lines()
	if len(lines) != 4 {
		t.Fatalf("calls: %v", lines)
	}
	if !strings.HasSuffix(lines[0], "rename-context kind-ks-a k8s-a") || !strings.HasSuffix(lines[1], "rename-context kind-ks-b k8s-b") {
		t.Fatalf("renames: %v", lines)
	}
	if lines[2] != "kubectl config view --flatten --raw" || !strings.HasPrefix(f.Calls[2].Env[0], "KUBECONFIG=") ||
		strings.Count(f.Calls[2].Env[0], string(os.PathListSeparator)) != 1 {
		t.Fatalf("merge: %v %v", lines[2], f.Calls[2].Env)
	}
	if lines[3] != "kubectl config --kubeconfig "+out+" use-context k8s-b" {
		t.Fatalf("use-context: %s", lines[3])
	}
	// The renames must apply to private copies, never to the admin files.
	if strings.Contains(lines[0], filepath.Join(dir, "a.kubeconfig")) {
		t.Fatal("rename ran against the admin kubeconfig")
	}
	if info, _ := os.Stat(out); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "home", "kubeconfig-*")); len(m) != 0 {
		t.Fatalf("temp copies left behind: %v", m)
	}
}

func TestGenerateEmptyRemovesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(out, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Generate(context.Background(), &execx.Fake{}, nil, "", out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("file must be removed")
	}
	if err := Generate(context.Background(), &execx.Fake{}, nil, "", out); err != nil {
		t.Fatalf("removing a missing file must succeed: %v", err)
	}
}

func TestGenerateErrors(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "kc")
	e := []Entry{{Admin: writeAdmin(t, dir, "a"), Cluster: "ks-a", Context: "c"}}
	ctx := context.Background()

	if err := Generate(ctx, &execx.Fake{}, []Entry{{Admin: filepath.Join(dir, "missing"), Cluster: "x", Context: "c"}}, "c", out); err == nil {
		t.Fatal("missing admin file")
	}
	f := &execx.Fake{}
	f.Respond([]string{"kubectl", "config", "--kubeconfig"}, execx.Result{ExitCode: 1, Stderr: []byte("no such context")}, nil)
	if err := Generate(ctx, f, e, "c", out); err == nil || !strings.Contains(err.Error(), "no such context") {
		t.Fatalf("rename failure: %v", err)
	}
	f = &execx.Fake{}
	f.Respond([]string{"kubectl", "config", "view"}, execx.Result{ExitCode: 1, Stderr: []byte("bad")}, nil)
	if err := Generate(ctx, f, e, "c", out); err == nil {
		t.Fatal("view failure")
	}
	f = &execx.Fake{}
	f.Respond([]string{"kubectl"}, execx.Result{}, errors.New("no kubectl"))
	if err := Generate(ctx, f, e, "c", out); err == nil {
		t.Fatal("kubectl missing")
	}
	f = &execx.Fake{}
	f.Respond([]string{"kubectl", "config", "view"}, execx.Result{}, errors.New("no kubectl"))
	if err := Generate(ctx, f, e, "c", out); err == nil {
		t.Fatal("view run failure")
	}
}

func TestKindContext(t *testing.T) {
	if KindContext("ks-x") != "kind-ks-x" {
		t.Fatal("context name")
	}
}
