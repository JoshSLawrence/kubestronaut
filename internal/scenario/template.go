package scenario

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"text/template"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
)

// maxClusterName keeps node hostnames (cluster name plus "-control-plane")
// under the 63 character DNS label limit.
const maxClusterName = 40

// ClusterName derives the kind cluster name for a scenario.
func ClusterName(pack, name string) string {
	full := "ks-" + pack + "-" + name
	if len(full) <= maxClusterName {
		return full
	}
	sum := sha256.Sum256([]byte(full))
	return full[:maxClusterName-9] + "-" + hex.EncodeToString(sum[:])[:8]
}

// TemplateData is what scenario files can reference.
type TemplateData struct {
	Context string
	Cluster string

	used map[string]bool
}

// UsedNodes returns the sorted node aliases referenced so far.
func (d *TemplateData) UsedNodes() []string {
	out := make([]string, 0, len(d.used))
	for a := range d.used {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// Render executes src as a template. Besides .Context and .Cluster it offers
// {{node "node01"}}, which yields the real node name.
func (d *TemplateData) Render(name string, src []byte) ([]byte, error) {
	if d.used == nil {
		d.used = map[string]bool{}
	}
	t, err := template.New(name).Option("missingkey=error").Funcs(template.FuncMap{
		"node": func(alias string) (string, error) {
			d.used[alias] = true
			return backend.NodeName(d.Cluster, alias)
		},
	}).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Context, Cluster string }{d.Context, d.Cluster}); err != nil {
		return nil, fmt.Errorf("%s: template: %w", name, err)
	}
	return buf.Bytes(), nil
}
