package kind

import (
	"os"
	"strings"
	"testing"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

func TestBuildConfigGolden(t *testing.T) {
	img, err := ResolveImage("", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(BuildConfig(backend.ClusterSpec{Workers: 1}, img))
	want, err := os.ReadFile("testdata/two-nodes.golden.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("config mismatch\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestBuildConfigPatchesAndWorkers(t *testing.T) {
	got := string(BuildConfig(backend.ClusterSpec{
		Workers:        2,
		KubeadmPatches: []string{"kind: ClusterConfiguration\napiServer:\n  extraArgs:\n    v: \"2\"\n"},
	}, "img"))
	if !strings.Contains(got, "kubeadmConfigPatches:\n  - \"kind: ClusterConfiguration\\napiServer") {
		t.Fatalf("patch not embedded as a quoted scalar:\n%s", got)
	}
	if strings.Count(got, "role: worker") != 2 || strings.Count(got, "role: control-plane") != 1 {
		t.Fatalf("roles:\n%s", got)
	}
}

func TestResolveImage(t *testing.T) {
	for _, v := range []string{"v1.35.8", "v1.36.4", "v1.37.0"} {
		img, err := ResolveImage(v, "")
		if err != nil || !strings.HasPrefix(img, "kindest/node:"+v+"@sha256:") || len(img) != len("kindest/node:"+v+"@sha256:")+64 {
			t.Errorf("%s: %q %v", v, img, err)
		}
	}
	if img, _ := ResolveImage("", ""); !strings.Contains(img, DefaultVersion) {
		t.Fatalf("default: %s", img)
	}
	if img, _ := ResolveImage("v9", "custom:1"); img != "custom:1" {
		t.Fatal("override must win")
	}
	if _, err := ResolveImage("v1.1.0", ""); err == nil || !strings.Contains(err.Error(), "--node-image") {
		t.Fatalf("unknown version must be actionable: %v", err)
	}
}

func TestBuildConfigDisableDefaultCNI(t *testing.T) {
	got := string(BuildConfig(backend.ClusterSpec{DisableDefaultCNI: true}, "img"))
	if !strings.Contains(got, "networking:\n  disableDefaultCNI: true\nnodes:\n") {
		t.Fatalf("networking block missing or misplaced:\n%s", got)
	}
	if got := string(BuildConfig(backend.ClusterSpec{}, "img")); strings.Contains(got, "networking") {
		t.Fatalf("default config must not mention networking:\n%s", got)
	}
}
