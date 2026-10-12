package scenario

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Validate statically checks a scenario against its pack. It returns every
// problem found, each prefixed with the scenario id.
func (s *Scenario) Validate(p *Pack) []error {
	var errs []error
	add := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("%s: %s", s.ID, fmt.Sprintf(format, a...)))
	}

	if s.APIVersion != APIVersion {
		add("apiVersion must be %q, got %q", APIVersion, s.APIVersion)
	}
	if s.Kind != KindScenario {
		add("kind must be %q, got %q", KindScenario, s.Kind)
	}
	if !kebab.MatchString(s.DirName) || !kebab.MatchString(s.Pack) {
		add("pack and scenario directory names must be lowercase kebab-case (DNS-1123) because they become cluster names")
	}
	if s.Metadata.Name != s.DirName {
		add("metadata.name %q must equal the directory name %q", s.Metadata.Name, s.DirName)
	}
	if s.Metadata.Title == "" {
		add("metadata.title is required")
	}
	switch s.Metadata.Difficulty {
	case "", "easy", "medium", "hard":
	default:
		add("metadata.difficulty %q must be easy, medium or hard", s.Metadata.Difficulty)
	}
	switch {
	case p == nil:
		add("pack %q not found", s.Pack)
	case !hasDomain(p, s.Metadata.Domain):
		add("metadata.domain %q is not a domain of pack %s", s.Metadata.Domain, s.Pack)
	}
	if s.Spec.Context == "" {
		add("spec.context is required")
	}
	if s.Spec.Weight < 0 {
		add("spec.weight must not be negative")
	}
	if s.Spec.PassPercent < 1 || s.Spec.PassPercent > 100 {
		add("spec.passPercent must be between 1 and 100")
	}
	if s.Spec.Cluster.Workers < 0 {
		add("spec.cluster.workers must not be negative")
	}
	for _, img := range s.Spec.Images {
		if strings.TrimSpace(img) == "" {
			add("spec.images contains an empty entry")
		}
	}

	errs = append(errs, s.validatePrompt()...)
	errs = append(errs, s.validateSetup()...)
	errs = append(errs, s.validateChecks()...)
	errs = append(errs, s.validateSetupPaths()...)
	errs = append(errs, s.validateSolution()...)
	errs = append(errs, s.validateNodes()...)
	return errs
}

func hasDomain(p *Pack, id string) bool {
	for _, d := range p.Spec.Domains {
		if d.ID == id {
			return true
		}
	}
	return false
}

func (s *Scenario) fileExists(name string) bool {
	_, err := fs.Stat(s.Files, name)
	return err == nil
}

func (s *Scenario) validatePrompt() []error {
	switch {
	case s.Spec.PromptFile == "":
		return []error{fmt.Errorf("%s: spec.prompt is required", s.ID)}
	case !s.fileExists(s.Spec.PromptFile):
		return []error{fmt.Errorf("%s: prompt file %q does not exist", s.ID, s.Spec.PromptFile)}
	case strings.TrimSpace(s.Prompt) == "":
		return []error{fmt.Errorf("%s: prompt file %q is empty", s.ID, s.Spec.PromptFile)}
	}
	return nil
}

func (s *Scenario) validateSetup() []error {
	var errs []error
	for i, st := range s.Spec.Setup {
		add := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("%s: setup[%d]: %s", s.ID, i, fmt.Sprintf(format, a...)))
		}
		n := 0
		if st.Script != "" {
			n++
		}
		if st.Manifest != "" {
			n++
		}
		if st.Wait != nil {
			n++
		}
		if n != 1 {
			add("exactly one of script, manifest or wait is required, got %d", n)
			continue
		}
		switch {
		case st.Script != "":
			if !s.fileExists(st.Script) {
				add("script %q does not exist", st.Script)
			}
		case st.Manifest != "":
			if !s.fileExists(st.Manifest) {
				add("manifest %q does not exist", st.Manifest)
			}
		default:
			if st.Wait.Ref.Kind == "" {
				add("wait.ref.kind is required")
			}
			f := st.Wait.For
			if f != "create" && f != "delete" && !strings.HasPrefix(f, "condition=") {
				add("wait.for %q must be create, delete or condition=<Name>", f)
			}
		}
	}
	return errs
}

func (s *Scenario) validateChecks() []error {
	var errs []error
	if len(s.Spec.Checks) == 0 {
		errs = append(errs, fmt.Errorf("%s: at least one check is required", s.ID))
	}
	seen := map[string]bool{}
	for i, c := range s.Spec.Checks {
		add := func(msg string) {
			errs = append(errs, fmt.Errorf("%s: checks[%d] (%s): %s", s.ID, i, c.ID, msg))
		}
		switch {
		case c.ID == "":
			add("id is required")
		case !kebab.MatchString(c.ID):
			add("id must be kebab-case")
		case seen[c.ID]:
			add("duplicate id")
		}
		seen[c.ID] = true
		if c.Description == "" {
			add("description is required")
		}
		if c.Type == "" {
			add("type is required")
		}
		if c.Weight < 1 {
			add("weight must be at least 1")
		}
	}
	for _, id := range s.Spec.Validation.PassBeforeSolution {
		if !seen[id] {
			errs = append(errs, fmt.Errorf("%s: validation.passBeforeSolution names unknown check %q", s.ID, id))
		}
	}
	return errs
}

func (s *Scenario) validateSolution() []error {
	var errs []error
	switch {
	case s.Spec.Solution.Script == "":
		errs = append(errs, fmt.Errorf("%s: spec.solution.script is required", s.ID))
	case !s.fileExists(s.Spec.Solution.Script):
		errs = append(errs, fmt.Errorf("%s: solution script %q does not exist", s.ID, s.Spec.Solution.Script))
	}
	if r := s.Spec.Solution.Readme; r != "" && !s.fileExists(r) {
		errs = append(errs, fmt.Errorf("%s: solution readme %q does not exist", s.ID, r))
	}
	return errs
}

// validateNodes checks that every node alias in templates and in node fields
// exists given spec.cluster.workers, and that manifests render.
func (s *Scenario) validateNodes() []error {
	var errs []error
	d := &TemplateData{Cluster: ClusterName(s.Pack, s.DirName), Context: s.Spec.Context}
	if err := s.walk(d, false, func(string, []byte) error { return nil }); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", s.ID, err))
	}
	aliases := map[string]bool{}
	for _, a := range s.UsedNodes {
		aliases[a] = true
	}
	for _, a := range d.UsedNodes() {
		aliases[a] = true
	}
	for _, c := range s.Spec.Checks {
		var peek struct {
			Node string `json:"node"`
		}
		if json.Unmarshal(c.Raw, &peek) == nil && peek.Node != "" {
			aliases[peek.Node] = true
		}
	}
	for a := range aliases {
		if a == backend.ControlPlaneAlias {
			continue
		}
		n, ok := backend.WorkerIndex(a)
		if !ok || n > s.Spec.Cluster.Workers {
			errs = append(errs, fmt.Errorf("%s: unknown node alias %q: the cluster has %d worker(s), so use controlplane or node01..%s",
				s.ID, a, s.Spec.Cluster.Workers, backend.WorkerAlias(max(s.Spec.Cluster.Workers, 1))))
		}
	}
	return errs
}

// Validate checks every scenario and reports load problems too.
func (c *Catalog) Validate() []error {
	errs := append([]error(nil), c.Problems...)
	for _, s := range c.Scenarios {
		errs = append(errs, s.Validate(c.Packs[s.Pack])...)
	}
	return errs
}

// CheckScriptPaths lists the files that script checks run, cleaned.
func (s *Scenario) CheckScriptPaths() []string {
	var out []string
	for _, c := range s.Spec.Checks {
		if c.Type != "script" {
			continue
		}
		var peek struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(c.Raw, &peek) == nil && peek.Path != "" {
			out = append(out, path.Clean(peek.Path))
		}
	}
	return out
}

// Overlaps reports whether a and b are the same path or one contains the other.
func Overlaps(a, b string) bool {
	a, b = path.Clean(a), path.Clean(b)
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") || a == "." || b == "."
}

// validateSetupPaths guards the cleanup that deletes setup files after setup:
// it must never be able to take a script check's file, or the whole directory.
func (s *Scenario) validateSetupPaths() []error {
	var errs []error
	scripts := s.CheckScriptPaths()
	for i, st := range s.Spec.Setup {
		for _, p := range []string{st.Script, st.Manifest} {
			if p == "" {
				continue
			}
			if c := path.Clean(p); c == "." || c == ".." || strings.HasPrefix(c, "../") || path.IsAbs(c) {
				errs = append(errs, fmt.Errorf("%s: setup[%d]: path %q must be inside the scenario directory and not the directory itself", s.ID, i, p))
				continue
			}
			for _, sc := range scripts {
				if Overlaps(p, sc) {
					errs = append(errs, fmt.Errorf("%s: setup[%d]: path %q overlaps the script check file %q, which setup cleanup would delete", s.ID, i, p, sc))
				}
			}
		}
	}
	return errs
}
