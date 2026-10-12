// Package kube is the Kubernetes API access the harness needs. The default
// implementation shells out to kubectl; tests use Fake.
package kube

import (
	"context"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// Ref selects zero or more objects. The json tags let scenario files embed it.
type Ref struct {
	Kind          string `json:"kind"`
	Name          string `json:"name,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	Selector      string `json:"selector,omitempty"`
	FieldSelector string `json:"fieldSelector,omitempty"`
	AllNamespaces bool   `json:"allNamespaces,omitempty"`
}

// Client reads and changes cluster state.
type Client interface {
	// Get returns zero objects (not an error) when nothing matches.
	Get(ctx context.Context, r Ref) ([]map[string]any, error)
	Apply(ctx context.Context, manifest []byte) error
	Exec(ctx context.Context, ns, pod, container string, argv []string) (execx.Result, error)
	CanI(ctx context.Context, as, verb, resource, sub, ns string) (bool, error)
	Wait(ctx context.Context, r Ref, forCond string, t time.Duration) error
}
