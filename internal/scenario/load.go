package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	packFile     = "pack.yaml"
	scenarioFile = "scenario.yaml"
)

// Catalog is every pack and scenario found in the layers.
type Catalog struct {
	Packs     map[string]*Pack
	Scenarios []*Scenario // sorted by ID
	// Problems are load errors that did not stop the rest from loading.
	Problems []error
}

// LoadCatalog scans layers in order; a later layer replaces a pack or a whole
// scenario directory of the same name from an earlier one.
func LoadCatalog(layers ...fs.FS) *Catalog {
	c := &Catalog{Packs: map[string]*Pack{}}
	byID := map[string]*Scenario{}

	type layerDir struct {
		fsys fs.FS
		pack string
	}
	var dirs []layerDir
	for _, l := range layers {
		entries, err := fs.ReadDir(l, ".")
		if err != nil {
			c.Problems = append(c.Problems, fmt.Errorf("read scenarios: %w", err))
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			raw, err := fs.ReadFile(l, path.Join(e.Name(), packFile))
			if err != nil {
				// A layer may add scenarios to a pack defined below it
				// without repeating pack.yaml.
				if _, known := c.Packs[e.Name()]; known {
					dirs = append(dirs, layerDir{l, e.Name()})
				}
				continue
			}
			var p Pack
			if err := decodeStrict(raw, &p); err != nil {
				c.Problems = append(c.Problems, fmt.Errorf("%s/%s: %w", e.Name(), packFile, err))
				continue
			}
			if p.APIVersion != APIVersion || p.Kind != KindPack {
				c.Problems = append(c.Problems, fmt.Errorf("%s/%s: apiVersion must be %q and kind %q", e.Name(), packFile, APIVersion, KindPack))
				continue
			}
			if !kebab.MatchString(e.Name()) {
				c.Problems = append(c.Problems, fmt.Errorf("%s: pack names must be lowercase kebab-case (DNS-1123) because they become cluster names", e.Name()))
				continue
			}
			if p.Metadata.Name != e.Name() {
				c.Problems = append(c.Problems, fmt.Errorf("%s/%s: metadata.name %q must equal the directory name", e.Name(), packFile, p.Metadata.Name))
				continue
			}
			if p.Spec.PassPercent == 0 {
				p.Spec.PassPercent = DefaultPassPercent
			}
			c.Packs[p.Metadata.Name] = &p
			dirs = append(dirs, layerDir{l, e.Name()})
		}
	}
	for _, ld := range dirs {
		entries, err := fs.ReadDir(ld.fsys, ld.pack)
		if err != nil {
			c.Problems = append(c.Problems, err)
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, err := fs.Stat(ld.fsys, path.Join(ld.pack, e.Name(), scenarioFile)); err != nil {
				continue // e.g. the exams directory
			}
			s, err := LoadScenario(ld.fsys, ld.pack, e.Name())
			if err != nil {
				c.Problems = append(c.Problems, err)
				continue
			}
			byID[s.ID] = s
		}
	}
	for _, s := range byID {
		c.Scenarios = append(c.Scenarios, s)
	}
	sort.Slice(c.Scenarios, func(i, j int) bool { return c.Scenarios[i].ID < c.Scenarios[j].ID })
	return c
}

// LoadScenario reads <pack>/<dir> from fsys, renders and decodes it.
func LoadScenario(fsys fs.FS, pack, dir string) (*Scenario, error) {
	id := pack + "/" + dir
	sub, err := fs.Sub(fsys, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	raw, err := fs.ReadFile(sub, scenarioFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	data := &TemplateData{Cluster: ClusterName(pack, dir)}

	// .Context usually comes from this very file, so a first lenient pass
	// finds it before the real render.
	if first, err := data.Render(id, raw); err == nil {
		var peek struct {
			Spec struct {
				Context string `json:"context"`
			} `json:"spec"`
		}
		if j, err := yaml.YAMLToJSON(first); err == nil && json.Unmarshal(j, &peek) == nil {
			data.Context = peek.Spec.Context
		}
	}
	data.used = nil
	rendered, err := data.Render(id+"/"+scenarioFile, raw)
	if err != nil {
		return nil, err
	}
	var s Scenario
	if err := decodeStrict(rendered, &s); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", id, scenarioFile, err)
	}
	s.ID, s.Pack, s.DirName, s.Files = id, pack, dir, sub
	if s.Spec.PromptFile != "" {
		if pb, err := fs.ReadFile(sub, s.Spec.PromptFile); err == nil {
			out, err := data.Render(id+"/"+s.Spec.PromptFile, pb)
			if err != nil {
				return nil, err
			}
			s.Prompt = string(out)
		}
	}
	s.UsedNodes = data.UsedNodes()
	applyDefaults(&s)
	return &s, nil
}

func applyDefaults(s *Scenario) {
	if s.Spec.PassPercent == 0 {
		s.Spec.PassPercent = DefaultPassPercent
	}
	for i := range s.Spec.Checks {
		c := &s.Spec.Checks[i]
		if c.Weight == 0 {
			c.Weight = 1
		}
		if c.Timeout == 0 {
			c.Timeout = Duration(DefaultCheckTimeout)
		}
		if c.Retry.Attempts < 1 {
			c.Retry.Attempts = 1
		}
	}
	for i := range s.Spec.Setup {
		st := &s.Spec.Setup[i]
		switch {
		case st.Wait != nil:
			// wait.timeout beats the step's timeout, which beats the default.
			if st.Wait.Timeout != 0 {
				st.Timeout = st.Wait.Timeout
			} else if st.Timeout == 0 {
				st.Timeout = Duration(DefaultWaitTimeout)
			}
		case st.Timeout == 0:
			st.Timeout = Duration(DefaultScriptTimeout)
		}
	}
}

// Find resolves "cka/name", or a name that is unique across packs.
func (c *Catalog) Find(ref string) (*Scenario, error) {
	var matches []*Scenario
	for _, s := range c.Scenarios {
		if s.ID == ref {
			return s, nil
		}
		if s.DirName == ref {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no scenario %q: run `kubestronaut list` to see the available ids", ref)
	case 1:
		return matches[0], nil
	}
	ids := make([]string, len(matches))
	for i, m := range matches {
		ids[i] = m.ID
	}
	return nil, fmt.Errorf("scenario %q is ambiguous (%s): use the full id", ref, strings.Join(ids, ", "))
}

// decodeStrict converts YAML to JSON and rejects unknown fields.
func decodeStrict(y []byte, v any) error {
	j, err := yaml.YAMLToJSON(y)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(j))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected trailing content")
	}
	return nil
}
