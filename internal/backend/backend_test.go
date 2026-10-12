package backend

import "testing"

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

func TestWorkerAlias(t *testing.T) {
	if WorkerAlias(3) != "node03" {
		t.Fatal(WorkerAlias(3))
	}
	if n, ok := WorkerIndex("node12"); !ok || n != 12 {
		t.Fatal("index")
	}
}
