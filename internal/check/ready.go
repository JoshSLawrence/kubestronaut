package check

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/kube"
)

func init() {
	Register("ready", func(raw json.RawMessage) (Check, error) {
		var c ready
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if err := validRef(c.Ref); err != nil {
			return nil, err
		}
		if c.MinReady < 0 {
			return nil, fmt.Errorf("minReady must not be negative")
		}
		k, ok := readyKinds[strings.ToLower(c.Ref.Kind)]
		if !ok {
			return nil, fmt.Errorf("ref.kind %q is not supported: use pod, deployment, statefulset, daemonset, node, pvc or job", c.Ref.Kind)
		}
		c.kind = k
		return &c, nil
	})
}

var readyKinds = map[string]string{
	"pod": "pod", "pods": "pod", "po": "pod",
	"deployment": "deployment", "deployments": "deployment", "deploy": "deployment",
	"statefulset": "statefulset", "statefulsets": "statefulset", "sts": "statefulset",
	"daemonset": "daemonset", "daemonsets": "daemonset", "ds": "daemonset",
	"node": "node", "nodes": "node", "no": "node",
	"persistentvolumeclaim": "pvc", "persistentvolumeclaims": "pvc", "pvc": "pvc",
	"job": "job", "jobs": "job",
}

type ready struct {
	Ref      kube.Ref `json:"ref"`
	MinReady int      `json:"minReady"`

	kind string
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func field(obj map[string]any, path ...string) any {
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

func conditionTrue(obj map[string]any, typ string) bool {
	conds, _ := field(obj, "status", "conditions").([]any)
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if m["type"] == typ {
			return m["status"] == "True"
		}
	}
	return false
}

// replicasOf returns the desired replica count, defaulting to 1 like the API.
func replicasOf(obj map[string]any) float64 {
	if v := field(obj, "spec", "replicas"); v != nil {
		return num(v)
	}
	return 1
}

// workloadReady judges a controller by replica counts. wantMin replaces the
// desired count when positive.
func workloadReady(obj map[string]any, kind string, wantMin int) (bool, string) {
	want := replicasOf(obj)
	if wantMin > 0 {
		want = float64(wantMin)
	}
	var have float64
	switch kind {
	case "daemonset":
		if wantMin == 0 {
			want = num(field(obj, "status", "desiredNumberScheduled"))
			if want == 0 {
				return false, "no pods scheduled yet"
			}
		}
		have = num(field(obj, "status", "numberReady"))
	case "statefulset":
		have = num(field(obj, "status", "readyReplicas"))
	default:
		have = num(field(obj, "status", "readyReplicas"))
		// A rollout that has not caught up is not ready even if old pods are.
		updated := num(field(obj, "status", "updatedReplicas"))
		observed := num(field(obj, "status", "observedGeneration"))
		generation := num(field(obj, "metadata", "generation"))
		if updated < want || observed < generation {
			return false, fmt.Sprintf("%.0f/%.0f replicas updated", updated, want)
		}
	}
	return have >= want, fmt.Sprintf("%.0f/%.0f replicas ready", have, want)
}

func objectReady(obj map[string]any, kind string, wantMin int) (bool, string) {
	switch kind {
	case "pod":
		return conditionTrue(obj, "Ready"), "not Ready"
	case "node":
		return conditionTrue(obj, "Ready"), "not Ready"
	case "pvc":
		phase, _ := field(obj, "status", "phase").(string)
		return phase == "Bound", "phase is " + phase
	case "job":
		return conditionTrue(obj, "Complete"), "not complete"
	}
	return workloadReady(obj, kind, wantMin)
}

func (c *ready) Run(ctx context.Context, env *Env) (Result, error) {
	objs, err := env.Kube.Get(ctx, c.Ref)
	if err != nil {
		return Result{}, err
	}
	// Terminating pods are on their way out and must not fail a check that
	// selects a whole workload.
	live := objs[:0:0]
	for _, o := range objs {
		if field(o, "metadata", "deletionTimestamp") == nil {
			live = append(live, o)
		}
	}
	if len(live) == 0 {
		return fail("%s not found", describe(c.Ref)), nil
	}
	workload := c.kind == "deployment" || c.kind == "statefulset" || c.kind == "daemonset"
	readyN := 0
	var firstMsg string
	for _, o := range live {
		ok, msg := objectReady(o, c.kind, c.MinReady)
		if ok {
			readyN++
		} else if firstMsg == "" {
			firstMsg = fmt.Sprintf("%v: %s", field(o, "metadata", "name"), msg)
		}
	}
	need := len(live)
	if c.MinReady > 0 && !workload {
		need = c.MinReady
	}
	if readyN < need {
		if firstMsg == "" {
			firstMsg = fmt.Sprintf("%d of %d ready", readyN, need)
		}
		return fail("%s: %s", describe(c.Ref), firstMsg), nil
	}
	return pass(), nil
}
