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

func TestBuildConfigThreeControlPlanesGolden(t *testing.T) {
	img, err := ResolveImage("", "")
	if err != nil {
		t.Fatal(err)
	}
	got := string(BuildConfig(backend.ClusterSpec{ControlPlanes: 3, Workers: 1}, img))
	want, err := os.ReadFile("testdata/three-cp.golden.yaml")
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

func TestBuildConfigMountsIntoEveryNode(t *testing.T) {
	got := string(buildConfig(backend.ClusterSpec{ControlPlanes: 3, Workers: 1}, "img",
		[]Mount{{HostPath: "/boot/config-6.1", ContainerPath: "/boot/config-6.1"}}))
	want := "    extraMounts:\n      - hostPath: \"/boot/config-6.1\"\n        containerPath: \"/boot/config-6.1\"\n        readOnly: true\n"
	if strings.Count(got, want) != 4 {
		t.Fatalf("each of the 4 nodes needs the mount:\n%s", got)
	}
	if strings.Contains(string(BuildConfig(backend.ClusterSpec{}, "img")), "extraMounts") {
		t.Fatal("BuildConfig must not add mounts")
	}
}

func TestKernelConfigMounts(t *testing.T) {
	release := func(string) ([]byte, error) { return []byte("6.17.0-1022-azure\n"), nil }
	exists := func(string) (os.FileInfo, error) { return nil, nil }
	missing := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	got := kernelConfigMounts("linux", "", release, exists)
	if len(got) != 1 || got[0].HostPath != "/boot/config-6.17.0-1022-azure" || got[0].ContainerPath != got[0].HostPath {
		t.Fatalf("linux host: %+v", got)
	}
	if got := kernelConfigMounts("linux", "unix:///var/run/docker.sock", release, exists); len(got) != 1 {
		t.Fatalf("a local socket is still this kernel: %+v", got)
	}
	for name, m := range map[string][]Mount{
		"darwin":        kernelConfigMounts("darwin", "", release, exists),
		"remote daemon": kernelConfigMounts("linux", "tcp://10.0.0.5:2376", release, exists),
		"no config":     kernelConfigMounts("linux", "", release, missing),
		"no osrelease": kernelConfigMounts("linux", "", func(string) ([]byte, error) {
			return nil, os.ErrNotExist
		}, exists),
	} {
		if m != nil {
			t.Errorf("%s: want no mounts, got %+v", name, m)
		}
	}
}
