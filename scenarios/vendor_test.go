package scenarios

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Vendored files are copies of third-party artifacts. The lock pins each by
// sha256, so a copy that was edited, truncated or reformatted fails here
// instead of surprising a learner.
func TestVendoredFilesMatchLock(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("cka", "vendor.lock"))
	if err != nil {
		t.Fatal(err)
	}
	repo := os.DirFS("..")
	entries := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 5 {
			t.Errorf("lock line needs name version sha256 url destination...: %q", line)
			continue
		}
		name, want, url, dests := f[0], f[2], f[3], f[4:]
		entries++
		if len(want) != 64 || !strings.HasPrefix(url, "https://") {
			t.Errorf("%s: malformed sha256 or url", name)
		}
		for _, d := range dests {
			if !strings.HasPrefix(d, "scenarios/cka/") || !strings.Contains(d, "/files/") {
				t.Errorf("%s: destination %q must be under a scenario's files/ directory", name, d)
				continue
			}
			data, err := fs.ReadFile(repo, d)
			if err != nil {
				t.Errorf("%s: %v: run `mise run vendor`", name, err)
				continue
			}
			sum := sha256.Sum256(data)
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("%s: %s has sha256 %s, the lock says %s: run `mise run vendor` to restore it", name, d, got, want)
			}
		}
	}
	if entries == 0 {
		t.Fatal("the lock lists nothing")
	}
}
