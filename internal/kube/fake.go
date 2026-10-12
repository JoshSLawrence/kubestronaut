package kube

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// Fake is an in-memory Client for tests.
type Fake struct {
	mu sync.Mutex
	// Objects are matched by Get on kind, name, namespace and selectors.
	Objects []map[string]any
	// GetErr, when set, makes Get fail.
	GetErr error
	// ExecFunc answers Exec; nil means exit 0 with no output.
	ExecFunc func(ns, pod, container string, argv []string) (execx.Result, error)
	// CanIAnswers maps "as|verb|resource|sub|ns" to the answer; missing keys
	// are an error so tests notice unexpected calls.
	CanIAnswers map[string]bool
	WaitErr     error

	Applied [][]byte
	Waited  []Ref
	GetRefs []Ref
}

// CanIKey builds a CanIAnswers key.
func CanIKey(as, verb, resource, sub, ns string) string {
	return strings.Join([]string{as, verb, resource, sub, ns}, "|")
}

var shortNames = map[string]string{
	"po": "pod", "no": "node", "deploy": "deployment", "sts": "statefulset",
	"ds": "daemonset", "pvc": "persistentvolumeclaim", "pv": "persistentvolume",
	"svc": "service", "ns": "namespace", "netpol": "networkpolicy",
}

func norm(kind string) string {
	k := strings.ToLower(kind)
	if long, ok := shortNames[k]; ok {
		k = long
	}
	switch {
	case strings.HasSuffix(k, "ies"):
		return strings.TrimSuffix(k, "ies") + "y"
	case strings.HasSuffix(k, "ss"):
		return k
	}
	return strings.TrimSuffix(k, "s")
}

func dig(obj map[string]any, path ...string) any {
	var cur any = obj
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func labelsMatch(obj map[string]any, selector string) bool {
	labels, _ := dig(obj, "metadata", "labels").(map[string]any)
	for _, term := range strings.Split(selector, ",") {
		term = strings.TrimSpace(term)
		switch {
		case term == "":
		case strings.Contains(term, "!="):
			kv := strings.SplitN(term, "!=", 2)
			if str(labels[kv[0]]) == kv[1] {
				return false
			}
		case strings.Contains(term, "="):
			kv := strings.SplitN(strings.ReplaceAll(term, "==", "="), "=", 2)
			if str(labels[kv[0]]) != kv[1] {
				return false
			}
		default:
			if _, ok := labels[term]; !ok {
				return false
			}
		}
	}
	return true
}

func fieldsMatch(obj map[string]any, selector string) bool {
	for _, term := range strings.Split(selector, ",") {
		kv := strings.SplitN(term, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if str(dig(obj, strings.Split(kv[0], ".")...)) != kv[1] {
			return false
		}
	}
	return true
}

func matches(obj map[string]any, r Ref) bool {
	if norm(str(obj["kind"])) != norm(r.Kind) {
		return false
	}
	if r.Name != "" && str(dig(obj, "metadata", "name")) != r.Name {
		return false
	}
	if !r.AllNamespaces {
		ns := str(dig(obj, "metadata", "namespace"))
		want := r.Namespace
		if want == "" {
			want = "default"
		}
		// Cluster-scoped objects carry no namespace.
		if ns != "" && ns != want {
			return false
		}
	}
	if r.Selector != "" && !labelsMatch(obj, r.Selector) {
		return false
	}
	return r.FieldSelector == "" || fieldsMatch(obj, r.FieldSelector)
}

// Get implements Client.
func (f *Fake) Get(_ context.Context, r Ref) ([]map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetRefs = append(f.GetRefs, r)
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	var out []map[string]any
	for _, o := range f.Objects {
		if matches(o, r) {
			out = append(out, o)
		}
	}
	return out, nil
}

// Apply implements Client.
func (f *Fake) Apply(_ context.Context, manifest []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Applied = append(f.Applied, manifest)
	return nil
}

// Exec implements Client.
func (f *Fake) Exec(_ context.Context, ns, pod, container string, argv []string) (execx.Result, error) {
	if f.ExecFunc == nil {
		return execx.Result{}, nil
	}
	return f.ExecFunc(ns, pod, container, argv)
}

// CanI implements Client.
func (f *Fake) CanI(_ context.Context, as, verb, resource, sub, ns string) (bool, error) {
	key := CanIKey(as, verb, resource, sub, ns)
	v, ok := f.CanIAnswers[key]
	if !ok {
		return false, fmt.Errorf("fake: no can-i answer for %q", key)
	}
	return v, nil
}

// Wait implements Client.
func (f *Fake) Wait(_ context.Context, r Ref, _ string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Waited = append(f.Waited, r)
	return f.WaitErr
}

var errFake = errors.New("fake failure")

// ErrFake is a reusable error for tests.
func ErrFake() error { return errFake }
