package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/grade"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	s := &Store{Root: filepath.Join(t.TempDir(), "home")}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
	dl := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	in := &State{
		Mode: ModeScenario, ID: "id1", StartedAt: dl, Deadline: &dl,
		Clusters: []ClusterRef{{Name: "ks-a", Context: "ctx", Nodes: []backend.Node{{Name: "n", Alias: "controlplane", Role: backend.RoleControlPlane}}}},
		Tasks:    []Task{{Scenario: "cka/x", Cluster: "ks-a", Weight: 7, LastGrade: &grade.Report{Earned: 1, Possible: 2}}},
	}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != SchemaVersion || out.Tasks[0].LastGrade.Earned != 1 || !out.Deadline.Equal(dl) || out.Clusters[0].Nodes[0].Alias != "controlplane" {
		t.Fatalf("round trip: %+v", out)
	}
	if c, ok := out.Cluster("ks-a"); !ok || c.Context != "ctx" {
		t.Fatal("Cluster lookup")
	}
	if _, ok := out.Cluster("nope"); ok {
		t.Fatal("unknown cluster")
	}
	info, _ := os.Stat(s.Root)
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("home mode %v", info.Mode().Perm())
	}
	// No temp files may linger after an atomic write.
	entries, _ := os.ReadDir(s.Root)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}

func TestLoadRejectsCorruptAndFutureVersions(t *testing.T) {
	s := &Store{Root: filepath.Join(t.TempDir(), "home")}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.statePath(), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "stop --all") {
		t.Fatalf("corrupt: %v", err)
	}
	if err := os.WriteFile(s.statePath(), []byte(`{"version": 99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "version 99") {
		t.Fatalf("version: %v", err)
	}
}

func TestClear(t *testing.T) {
	s := &Store{Root: filepath.Join(t.TempDir(), "home")}
	if err := s.Clear(); err != nil {
		t.Fatalf("clearing nothing must succeed: %v", err)
	}
	if err := s.Save(&State{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("after clear: %v", err)
	}
}

func TestLockIsExclusive(t *testing.T) {
	s := &Store{Root: filepath.Join(t.TempDir(), "home")}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lock(); err == nil || !strings.Contains(err.Error(), "another kubestronaut command") {
		t.Fatalf("second lock must fail: %v", err)
	}
	unlock()
	again, err := s.Lock()
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	again()
}

func TestPathsAndIDs(t *testing.T) {
	s := &Store{Root: "/h"}
	if s.KubeconfigPath() != "/h/kubeconfig" || s.SessionDir("x") != "/h/sessions/x" || s.ScratchKubeconfig() != "/h/scratch.kubeconfig" {
		t.Fatal("paths")
	}
	a := NewID(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	b := NewID(time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC))
	if !strings.HasPrefix(a, "20260101T000000Z-") || a >= b {
		t.Fatalf("ids must sort by time: %s %s", a, b)
	}
}

func TestInitFailure(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Store{Root: filepath.Join(f, "sub")}
	if err := s.Save(&State{}); err == nil {
		t.Fatal("want error")
	}
	if _, err := s.Lock(); err == nil {
		t.Fatal("want error")
	}
}

func TestInitSafety(t *testing.T) {
	base := t.TempDir()
	mode := func(p string) os.FileMode {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Mode().Perm()
	}

	fresh := filepath.Join(base, "fresh")
	if err := (&Store{Root: fresh}).Init(); err != nil || mode(fresh) != 0o700 {
		t.Fatalf("new dir: %v mode %v", err, mode(fresh))
	}
	// Idempotent on the directory it created.
	if err := (&Store{Root: fresh}).Init(); err != nil {
		t.Fatal(err)
	}

	open := filepath.Join(base, "open")
	if err := os.Mkdir(open, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Chmod(open, 0o750); err != nil { // defeat the umask
		t.Fatal(err)
	}
	err := (&Store{Root: open}).Init()
	if err == nil || !strings.Contains(err.Error(), "chmod 700") || mode(open) != 0o750 {
		t.Fatalf("accessible dir must be refused and left alone: %v mode %v", err, mode(open))
	}

	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := (&Store{Root: link}).Init(); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink: %v", err)
	}

	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Store{Root: file}).Init(); err == nil {
		t.Fatal("a file is not a state dir")
	}

	for _, bad := range []string{"/", os.TempDir()} {
		if err := (&Store{Root: bad}).Init(); err == nil || !strings.Contains(err.Error(), "dedicated") {
			t.Fatalf("%s must be refused: %v", bad, err)
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		if err := (&Store{Root: h}).Init(); err == nil {
			t.Fatal("$HOME must be refused")
		}
	}
}
