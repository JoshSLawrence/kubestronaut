package scenario

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// templated reports whether a file is rendered when materialized. Scripts are
// never templated: they receive environment variables instead.
func templated(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".yaml", ".yml", ".json":
		return true
	}
	return false
}

// walk visits the scenario's files (rendered if templated), optionally only
// those under the solution directory, or everything except it.
func (s *Scenario) walk(d *TemplateData, solution bool, fn func(rel string, data []byte) error) error {
	return fs.WalkDir(s.Files, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		inSolution := p == "solution" || strings.HasPrefix(p, "solution/")
		if inSolution != solution {
			return nil
		}
		b, err := fs.ReadFile(s.Files, p)
		if err != nil {
			return err
		}
		if templated(p) {
			if b, err = d.Render(s.ID+"/"+p, b); err != nil {
				return err
			}
		}
		return fn(p, b)
	})
}

func writeTree(dst string) func(rel string, data []byte) error {
	return func(rel string, data []byte) error {
		out := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o600)
	}
}

// Materialize writes the scenario (without solution/) to dst, rendering
// manifests with d. Solution files are kept out so the learner cannot stumble
// on them in the session directory.
func (s *Scenario) Materialize(dst string, d *TemplateData) error {
	if err := s.walk(d, false, writeTree(dst)); err != nil {
		return fmt.Errorf("materialize %s: %w", s.ID, err)
	}
	return nil
}

// MaterializeSolution writes solution/ to dst/solution.
func (s *Scenario) MaterializeSolution(dst string, d *TemplateData) error {
	if err := s.walk(d, true, writeTree(dst)); err != nil {
		return fmt.Errorf("materialize solution %s: %w", s.ID, err)
	}
	return nil
}
