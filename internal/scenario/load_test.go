package scenario

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const packYAML = `apiVersion: kubestronaut/v1alpha1
kind: Pack
metadata: {name: cka}
spec:
  title: CKA
  passPercent: 66
  domains:
    - {id: troubleshooting, title: Troubleshooting, weight: 30}
`

const goodScenario = `apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata:
  name: demo
  title: Demo
  domain: troubleshooting
  difficulty: easy
spec:
  timeLimit: 10m
  context: k8s-{{.Cluster}}
  cluster: {workers: 1}
  prompt: prompt.md
  hints: ["look at {{node "node01"}}"]
  setup:
    - script: setup/s.sh
    - manifest: setup/m.yaml
    - wait:
        ref: {kind: deployment, name: web, namespace: ops}
        for: condition=Available
  checks:
    - id: node-ready
      description: node is ready
      type: ready
      ref: {kind: node, name: '{{node "node01"}}'}
    - id: exec
      description: exec
      type: nodeExec
      node: controlplane
      command: [true]
      weight: 3
  solution:
    script: solution/solve.sh
    readme: solution/README.md
  validation:
    passBeforeSolution: [exec]
`

func goodFS() fstest.MapFS {
	return fstest.MapFS{
		"cka/pack.yaml":                {Data: []byte(packYAML)},
		"cka/demo/scenario.yaml":       {Data: []byte(goodScenario)},
		"cka/demo/prompt.md":           {Data: []byte("Fix {{node \"node01\"}} in {{.Context}}\n")},
		"cka/demo/setup/s.sh":          {Data: []byte("echo {{ not template }}\n")},
		"cka/demo/setup/m.yaml":        {Data: []byte("name: {{node \"controlplane\"}}\n")},
		"cka/demo/solution/solve.sh":   {Data: []byte("true\n")},
		"cka/demo/solution/README.md":  {Data: []byte("# sol\n")},
		"cka/exams/mock.yaml":          {Data: []byte("ignored: true\n")},
		"cka/other/not-a-scenario.txt": {Data: []byte("x")},
		"stray-file.txt":               {Data: []byte("x")},
		"notapack/readme.md":           {Data: []byte("x")},
	}
}

func mustLoad(t *testing.T, fsys fstest.MapFS) *Catalog {
	t.Helper()
	c := LoadCatalog(fsys)
	if len(c.Problems) != 0 {
		t.Fatalf("problems: %v", c.Problems)
	}
	return c
}

func TestLoadAndRender(t *testing.T) {
	c := mustLoad(t, goodFS())
	if len(c.Scenarios) != 1 {
		t.Fatalf("scenarios: %d", len(c.Scenarios))
	}
	s := c.Scenarios[0]
	if s.ID != "cka/demo" || s.Spec.Context != "k8s-ks-cka-demo" {
		t.Fatalf("id %q context %q", s.ID, s.Spec.Context)
	}
	if s.Spec.Hints[0] != "look at ks-cka-demo-worker" {
		t.Fatalf("hint: %q", s.Spec.Hints[0])
	}
	if !strings.Contains(s.Prompt, "Fix ks-cka-demo-worker in k8s-ks-cka-demo") {
		t.Fatalf("prompt: %q", s.Prompt)
	}
	if s.Spec.PassPercent != 100 || s.Spec.Checks[0].Weight != 1 || s.Spec.Checks[1].Weight != 3 {
		t.Fatalf("defaults not applied: %+v", s.Spec)
	}
	if s.Spec.Checks[0].Timeout.D() != DefaultCheckTimeout || s.Spec.Checks[0].Retry.Attempts != 1 {
		t.Fatalf("check defaults: %+v", s.Spec.Checks[0])
	}
	if s.Spec.Setup[0].Timeout.D() != DefaultScriptTimeout || s.Spec.Setup[2].Timeout.D() != DefaultWaitTimeout {
		t.Fatalf("setup defaults: %+v", s.Spec.Setup)
	}
	if got := strings.Join(s.UsedNodes, ","); got != "node01" {
		t.Fatalf("used nodes: %s", got)
	}
	if errs := c.Validate(); len(errs) != 0 {
		t.Fatalf("validate: %v", errs)
	}
	if c.Packs["cka"].Spec.PassPercent != 66 {
		t.Fatal("pack passPercent")
	}
}

func TestFind(t *testing.T) {
	other := goodFS()
	other["cka/demo2/scenario.yaml"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(goodScenario, "name: demo", "name: demo2"))}
	other["cka/demo2/prompt.md"] = other["cka/demo/prompt.md"]
	other["ckad/pack.yaml"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(packYAML, "name: cka", "name: ckad"))}
	other["ckad/demo/scenario.yaml"] = other["cka/demo/scenario.yaml"]
	c := mustLoad(t, other)
	if s, err := c.Find("cka/demo"); err != nil || s.ID != "cka/demo" {
		t.Fatalf("exact: %v", err)
	}
	if s, err := c.Find("demo2"); err != nil || s.ID != "cka/demo2" {
		t.Fatalf("unique suffix: %v", err)
	}
	if _, err := c.Find("demo"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want ambiguous, got %v", err)
	}
	if _, err := c.Find("nope"); err == nil || !strings.Contains(err.Error(), "kubestronaut list") {
		t.Fatalf("want actionable not-found, got %v", err)
	}
}

func TestLayerOverride(t *testing.T) {
	over := fstest.MapFS{
		"cka/demo/scenario.yaml": {Data: []byte(strings.ReplaceAll(goodScenario, "title: Demo", "title: Overridden"))},
		"cka/demo/prompt.md":     {Data: []byte("new prompt")},
	}
	c := LoadCatalog(goodFS(), over)
	if len(c.Scenarios) != 1 {
		t.Fatalf("want 1 scenario, got %d (%v)", len(c.Scenarios), c.Problems)
	}
	if c.Scenarios[0].Metadata.Title != "Overridden" || c.Scenarios[0].Prompt != "new prompt" {
		t.Fatalf("override not applied: %+v", c.Scenarios[0].Metadata)
	}
	// The override directory replaces the whole scenario: setup files vanish.
	if c.Scenarios[0].fileExists("setup/s.sh") {
		t.Fatal("overridden scenario must not inherit files from the lower layer")
	}
}

func TestStrictDecodeRejectsUnknownFields(t *testing.T) {
	f := goodFS()
	f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(goodScenario, "spec:\n", "spec:\n  bogus: 1\n", 1))}
	c := LoadCatalog(f)
	if len(c.Problems) != 1 || !strings.Contains(c.Problems[0].Error(), "unknown field") {
		t.Fatalf("problems: %v", c.Problems)
	}
}

func TestClusterNetworkingDecodes(t *testing.T) {
	f := goodFS()
	f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(goodScenario, "cluster: {workers: 1}", "cluster: {workers: 1, networking: {disableDefaultCNI: true}}", 1))}
	c := LoadCatalog(f)
	if len(c.Problems) != 0 || !c.Scenarios[0].Spec.Cluster.Networking.DisableDefaultCNI {
		t.Fatalf("problems: %v", c.Problems)
	}
	f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(goodScenario, "cluster: {workers: 1}", "cluster: {networking: {bogus: true}}", 1))}
	if c := LoadCatalog(f); len(c.Problems) != 1 || !strings.Contains(c.Problems[0].Error(), "unknown field") {
		t.Fatalf("networking must be strictly decoded: %v", c.Problems)
	}
}

func TestLoadProblems(t *testing.T) {
	tests := map[string]fstest.MapFS{
		"bad pack yaml":    {"cka/pack.yaml": {Data: []byte("kind: [")}},
		"pack name":        {"cka/pack.yaml": {Data: []byte(strings.ReplaceAll(packYAML, "name: cka", "name: other"))}},
		"bad template":     {"cka/pack.yaml": {Data: []byte(packYAML)}, "cka/x/scenario.yaml": {Data: []byte("a: {{")}},
		"unknown alias fn": {"cka/pack.yaml": {Data: []byte(packYAML)}, "cka/x/scenario.yaml": {Data: []byte("a: '{{node \"bogus\"}}'")}},
		"bad duration":     {"cka/pack.yaml": {Data: []byte(packYAML)}, "cka/x/scenario.yaml": {Data: []byte("spec: {timeLimit: soon}")}},
		"non string dur":   {"cka/pack.yaml": {Data: []byte(packYAML)}, "cka/x/scenario.yaml": {Data: []byte("spec: {timeLimit: 5}")}},
		"trailing":         {"cka/pack.yaml": {Data: []byte(packYAML)}, "cka/x/scenario.yaml": {Data: []byte("{}")}},
	}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			c := LoadCatalog(f)
			if name == "trailing" {
				// An empty scenario loads but must fail validation.
				if errs := c.Validate(); len(errs) == 0 {
					t.Fatal("want validation errors")
				}
				return
			}
			if len(c.Problems) == 0 {
				t.Fatal("want a load problem")
			}
		})
	}
	if c := LoadCatalog(fstest.MapFS{}); len(c.Scenarios) != 0 || len(c.Problems) != 0 {
		t.Fatal("empty layer must load cleanly")
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(m fstest.MapFS) string // returns new scenario.yaml text
		want   string
	}{
		{"wrong dir name", func(fstest.MapFS) string { return strings.Replace(goodScenario, "name: demo", "name: other", 1) }, "must equal the directory name"},
		{"unknown domain", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "domain: troubleshooting", "domain: nope", 1)
		}, "not a domain"},
		{"bad difficulty", func(fstest.MapFS) string { return strings.Replace(goodScenario, "easy", "brutal", 1) }, "difficulty"},
		{"duplicate id", func(fstest.MapFS) string { return strings.Replace(goodScenario, "id: exec", "id: node-ready", 1) }, "duplicate id"},
		{"non kebab id", func(fstest.MapFS) string { return strings.Replace(goodScenario, "id: exec", "id: Not_Kebab", 1) }, "kebab-case"},
		{"unknown node alias", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "node: controlplane", "node: node03", 1)
		}, "unknown node alias"},
		{"worker without workers", func(fstest.MapFS) string { return strings.Replace(goodScenario, "workers: 1", "workers: 0", 1) }, "unknown node alias"},
		{"solution missing", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "script: solution/solve.sh", "script: solution/nope.sh", 1)
		}, "solution script"},
		{"solution unset", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "    script: solution/solve.sh\n", "", 1)
		}, "spec.solution.script is required"},
		{"readme missing", func(fstest.MapFS) string { return strings.Replace(goodScenario, "README.md", "NOPE.md", 1) }, "solution readme"},
		{"passBefore unknown", func(fstest.MapFS) string { return strings.Replace(goodScenario, "[exec]", "[ghost]", 1) }, `unknown check "ghost"`},
		{"setup script missing", func(fstest.MapFS) string { return strings.Replace(goodScenario, "setup/s.sh", "setup/gone.sh", 1) }, `script "setup/gone.sh"`},
		{"setup manifest missing", func(fstest.MapFS) string { return strings.Replace(goodScenario, "setup/m.yaml", "setup/gone.yaml", 1) }, "manifest"},
		{"setup two keys", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "    - script: setup/s.sh\n", "    - script: setup/s.sh\n      manifest: setup/m.yaml\n", 1)
		}, "exactly one"},
		{"wait bad for", func(fstest.MapFS) string { return strings.Replace(goodScenario, "condition=Available", "ready", 1) }, "wait.for"},
		{"wait no kind", func(fstest.MapFS) string { return strings.Replace(goodScenario, "kind: deployment, ", "", 1) }, "wait.ref.kind"},
		{"prompt missing", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "prompt: prompt.md", "prompt: gone.md", 1)
		}, "does not exist"},
		{"prompt unset", func(fstest.MapFS) string { return strings.Replace(goodScenario, "  prompt: prompt.md\n", "", 1) }, "spec.prompt is required"},
		{"no checks", func(fstest.MapFS) string {
			i := strings.Index(goodScenario, "  checks:")
			j := strings.Index(goodScenario, "  solution:")
			return goodScenario[:i] + "  checks: []\n" + goodScenario[j:]
		}, "at least one check"},
		{"no description", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "description: exec", "description: ''", 1)
		}, "description is required"},
		{"no type", func(fstest.MapFS) string { return strings.Replace(goodScenario, "type: nodeExec", "type: ''", 1) }, "type is required"},
		{"api version", func(fstest.MapFS) string { return strings.Replace(goodScenario, "v1alpha1", "v2", 1) }, "apiVersion"},
		{"kind", func(fstest.MapFS) string { return strings.Replace(goodScenario, "kind: Scenario", "kind: Thing", 1) }, "kind must be"},
		{"pass percent", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "spec:\n", "spec:\n  passPercent: 101\n", 1)
		}, "passPercent"},
		{"two control planes", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "workers: 1", "controlPlanes: 2, workers: 1", 1)
		}, "controlPlanes is 2, but must be 1 or 3"},
		{"four control planes", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "workers: 1", "controlPlanes: 4, workers: 1", 1)
		}, "must be 1 or 3"},
		{"control plane alias without HA", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "node: controlplane", "node: controlplane02", 1)
		}, "unknown node alias"},
		{"control plane alias past the last", func(fstest.MapFS) string {
			s := strings.Replace(goodScenario, "workers: 1", "controlPlanes: 3, workers: 1", 1)
			return strings.Replace(s, "node: controlplane", "node: controlplane04", 1)
		}, "controlplane or controlplane02..controlplane03"},
		{"first control plane under another name", func(fstest.MapFS) string {
			s := strings.Replace(goodScenario, "workers: 1", "controlPlanes: 3, workers: 1", 1)
			return strings.Replace(s, "node: controlplane", "node: controlplane01", 1)
		}, "unknown node alias"},
		{"negative workers", func(fstest.MapFS) string { return strings.Replace(goodScenario, "workers: 1", "workers: -1", 1) }, "workers must not be negative"},
		{"empty image", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "spec:\n", "spec:\n  images: ['']\n", 1)
		}, "empty entry"},
		{"no context", func(fstest.MapFS) string {
			return strings.Replace(goodScenario, "  context: k8s-{{.Cluster}}\n", "", 1)
		}, "spec.context is required"},
		{"no title", func(fstest.MapFS) string { return strings.Replace(goodScenario, "title: Demo", "title: ''", 1) }, "metadata.title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := goodFS()
			f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(tc.mutate(f))}
			c := LoadCatalog(f)
			errs := c.Validate()
			if !anyContains(errs, tc.want) {
				t.Fatalf("want an error containing %q, got %v", tc.want, errs)
			}
		})
	}
}

func TestValidateMissingPackAndBadManifest(t *testing.T) {
	f := goodFS()
	delete(f, "cka/pack.yaml")
	f["other/pack.yaml"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(packYAML, "name: cka", "name: other"))}
	f["other/demo/scenario.yaml"] = f["cka/demo/scenario.yaml"]
	f["other/demo/prompt.md"] = f["cka/demo/prompt.md"]
	c := LoadCatalog(f)
	s := c.Scenarios[0]
	if errs := s.Validate(nil); !anyContains(errs, "pack") {
		t.Fatalf("want pack error: %v", errs)
	}
	g := goodFS()
	g["cka/demo/setup/m.yaml"] = &fstest.MapFile{Data: []byte("name: {{node \"nodeX\"}}\n")}
	if errs := LoadCatalog(g).Validate(); !anyContains(errs, "unknown node alias") {
		t.Fatalf("want alias error from manifest: %v", errs)
	}
}

func TestMaterialize(t *testing.T) {
	c := mustLoad(t, goodFS())
	s := c.Scenarios[0]
	d := &TemplateData{Cluster: "ks-cka-demo", Context: "ctx"}
	dst := t.TempDir()
	if err := s.Materialize(dst, d); err != nil {
		t.Fatal(err)
	}
	m, err := fs.ReadFile(os.DirFS(dst), "setup/m.yaml")
	if err != nil || string(m) != "name: ks-cka-demo-control-plane\n" {
		t.Fatalf("manifest %q err %v", m, err)
	}
	sh, _ := fs.ReadFile(os.DirFS(dst), "setup/s.sh")
	if string(sh) != "echo {{ not template }}\n" {
		t.Fatalf("scripts must not be templated: %q", sh)
	}
	if _, err := os.Stat(filepath.Join(dst, "solution")); err == nil {
		t.Fatal("solution/ must not be materialized with the scenario")
	}
	sol := t.TempDir()
	if err := s.MaterializeSolution(sol, d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sol, "solution", "solve.sh")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sol, "setup")); err == nil {
		t.Fatal("only solution files expected")
	}
	if err := s.Materialize(filepath.Join(dst, "setup", "s.sh", "x"), d); err == nil {
		t.Fatal("want error writing below a file")
	}
}

func TestVerbatimFilesAreNotTemplated(t *testing.T) {
	fsys := goodFS()
	chart := "{{ .Values.image }} {{ node \"nope\" }}\n"
	fsys["cka/demo/files/chart/templates/deploy.yaml"] = &fstest.MapFile{Data: []byte(chart)}
	fsys["cka/demo/files/data.json"] = &fstest.MapFile{Data: []byte("{{ broken\n")}
	c := mustLoad(t, fsys)
	s := c.Scenarios[0]
	if errs := s.Validate(c.Packs["cka"]); len(errs) != 0 {
		t.Fatalf("files/ must not be rendered during validation: %v", errs)
	}
	dst := t.TempDir()
	if err := s.Materialize(dst, &TemplateData{Cluster: "ks-cka-demo", Context: "ctx"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Clean(filepath.Join(dst, "files", "chart", "templates", "deploy.yaml")))
	if err != nil || string(got) != chart {
		t.Fatalf("verbatim file changed: %q %v", got, err)
	}
	// Only the directory named files at the scenario root is verbatim.
	if templated("files/x.yaml") || templated("files") || !templated("setup/files/x.yaml") || !templated("filesx/a.yaml") {
		t.Fatal("verbatim applies to the files/ subtree at the scenario root only")
	}
}

func TestLoadBalancerNameFitsADNSLabel(t *testing.T) {
	// kind names the HA load balancer <cluster>-external-load-balancer, and a
	// container hostname is a DNS label of at most 63 characters.
	if n := maxClusterName + len("-external-load-balancer"); n > 63 {
		t.Fatalf("a %d character cluster name makes a %d character load balancer name", maxClusterName, n)
	}
}

func TestClusterName(t *testing.T) {
	if got := ClusterName("cka", "rbac"); got != "ks-cka-rbac" {
		t.Fatal(got)
	}
	long := ClusterName("cka", strings.Repeat("a", 80))
	if len(long) > maxClusterName || !strings.HasPrefix(long, "ks-cka-aaa") {
		t.Fatalf("long name %q (%d)", long, len(long))
	}
	if long == ClusterName("cka", strings.Repeat("a", 79)+"b") {
		t.Fatal("hash must distinguish truncated names")
	}
}

func TestDurationMarshal(t *testing.T) {
	b, err := Duration(90e9).MarshalJSON()
	if err != nil || string(b) != `"1m30s"` {
		t.Fatalf("%s %v", b, err)
	}
}

func anyContains(errs []error, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), sub) {
			return true
		}
	}
	return false
}

func TestWaitTimeoutDefaults(t *testing.T) {
	y := strings.Replace(goodScenario, "        for: condition=Available\n", "        for: condition=Available\n        timeout: 45s\n", 1)
	f := goodFS()
	f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(y)}
	c := mustLoad(t, f)
	if got := c.Scenarios[0].Spec.Setup[2].Timeout.D(); got != 45*time.Second {
		t.Fatalf("wait.timeout must win over the default, got %s", got)
	}
}

func TestNamesMustBeKebab(t *testing.T) {
	f := goodFS()
	f["Cka/pack.yaml"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(packYAML, "name: cka", "name: Cka"))}
	if c := LoadCatalog(f); !anyContains(c.Problems, "kebab-case") {
		t.Fatalf("problems: %v", c.Problems)
	}
	g := goodFS()
	g["cka/pack.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(packYAML, "kind: Pack", "kind: Nope", 1))}
	if c := LoadCatalog(g); !anyContains(c.Problems, "apiVersion must be") {
		t.Fatalf("problems: %v", c.Problems)
	}
	h := goodFS()
	h["cka/Bad_Name/scenario.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(goodScenario, "name: demo", "name: Bad_Name", 1))}
	h["cka/Bad_Name/prompt.md"] = h["cka/demo/prompt.md"]
	if errs := LoadCatalog(h).Validate(); !anyContains(errs, "kebab-case") {
		t.Fatalf("errs: %v", errs)
	}
}

func TestSetupPathsMustNotCoverCheckScripts(t *testing.T) {
	check := "    - id: scr\n      description: s\n      type: script\n      path: setup/check.sh\n"
	y := strings.Replace(goodScenario, "  checks:\n", "  checks:\n"+check, 1)
	y = strings.Replace(y, "manifest: setup/m.yaml", "manifest: setup", 1)
	f := goodFS()
	f["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(y)}
	f["cka/demo/setup/check.sh"] = &fstest.MapFile{Data: []byte("true")}
	if errs := LoadCatalog(f).Validate(); !anyContains(errs, "overlaps the script check") {
		t.Fatalf("errs: %v", errs)
	}
	dot := strings.Replace(goodScenario, "script: setup/s.sh", "script: .", 1)
	g := goodFS()
	g["cka/demo/scenario.yaml"] = &fstest.MapFile{Data: []byte(dot)}
	if errs := LoadCatalog(g).Validate(); !anyContains(errs, "not the directory itself") {
		t.Fatalf("errs: %v", errs)
	}
	if !Overlaps("setup", "setup/a.sh") || !Overlaps("a/b", "a/b") || Overlaps("setup/a.sh", "setup/b.sh") || !Overlaps(".", "x") {
		t.Fatal("Overlaps")
	}
}
