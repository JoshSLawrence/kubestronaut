package backend

import (
	"strings"
	"testing"
)

func TestNodeName(t *testing.T) {
	cases := map[string]string{
		"controlplane": "ks-x-control-plane",
		"node01":       "ks-x-worker",
		"node02":       "ks-x-worker2",
		"node10":       "ks-x-worker10",
	}
	for alias, want := range cases {
		if got, err := NodeName("ks-x", alias); err != nil || got != want {
			t.Errorf("%s: got %q err %v want %q", alias, got, err, want)
		}
	}
	for _, bad := range []string{"", "node", "node1", "node00", "worker", "node0a"} {
		if _, err := NodeName("ks-x", bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestControlPlaneAliases(t *testing.T) {
	cases := map[string]string{
		"controlplane":   "ks-x-control-plane",
		"controlplane02": "ks-x-control-plane2",
		"controlplane03": "ks-x-control-plane3",
		"controlplane10": "ks-x-control-plane10",
	}
	for alias, want := range cases {
		if got, err := NodeName("ks-x", alias); err != nil || got != want {
			t.Errorf("%s: got %q err %v want %q", alias, got, err, want)
		}
		n, ok := ControlPlaneIndex(alias)
		if !ok || ControlPlaneAliasN(n) != alias {
			t.Errorf("%s: index %d %v does not round trip to %q", alias, n, ok, ControlPlaneAliasN(n))
		}
	}
	if n, ok := ControlPlaneIndex("controlplane"); !ok || n != 1 {
		t.Fatalf("controlplane is the first control plane: %d %v", n, ok)
	}
	for _, bad := range []string{"controlplane01", "controlplane2", "controlplane00", "controlplane0a", "control-plane", "controlplane-02"} {
		if _, ok := ControlPlaneIndex(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
		if _, err := NodeName("ks-x", bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	_, err := NodeName("ks-x", "controlplane01")
	if err == nil || !strings.Contains(err.Error(), `just "controlplane"`) {
		t.Fatalf("controlplane01 must say what to use: %v", err)
	}
	if ControlPlaneAliasN(1) != "controlplane" || ControlPlaneAliasN(3) != "controlplane03" {
		t.Fatal("alias formatting")
	}
}

func TestWorkerAlias(t *testing.T) {
	if WorkerAlias(3) != "node03" {
		t.Fatal(WorkerAlias(3))
	}
	if n, ok := WorkerIndex("node12"); !ok || n != 12 {
		t.Fatal("index")
	}
}
