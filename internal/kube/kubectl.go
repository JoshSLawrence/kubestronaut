package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

// Kubectl implements Client by running the kubectl binary.
type Kubectl struct {
	Runner     execx.Runner
	Kubeconfig string
	// Binary defaults to "kubectl".
	Binary string
}

func (k *Kubectl) run(ctx context.Context, stdin []byte, args ...string) (execx.Result, error) {
	bin := k.Binary
	if bin == "" {
		bin = "kubectl"
	}
	c := execx.Cmd{Name: bin, Args: append([]string{"--kubeconfig", k.Kubeconfig}, args...)}
	if stdin != nil {
		c.Stdin = bytes.NewReader(stdin)
	}
	return k.Runner.Run(ctx, c)
}

func refArgs(r Ref) []string {
	args := []string{r.Kind}
	if r.Name != "" {
		args = append(args, r.Name)
	}
	switch {
	case r.AllNamespaces:
		args = append(args, "-A")
	case r.Namespace != "":
		args = append(args, "-n", r.Namespace)
	}
	if r.Selector != "" {
		args = append(args, "-l", r.Selector)
	}
	if r.FieldSelector != "" {
		args = append(args, "--field-selector", r.FieldSelector)
	}
	return args
}

func cmdErr(what string, res execx.Result) error {
	msg := strings.TrimSpace(string(res.Stderr))
	if msg == "" {
		msg = fmt.Sprintf("exit code %d", res.ExitCode)
	}
	return fmt.Errorf("kubectl %s: %s", what, msg)
}

// Get implements Client.
func (k *Kubectl) Get(ctx context.Context, r Ref) ([]map[string]any, error) {
	res, err := k.run(ctx, nil, append(append([]string{"get"}, refArgs(r)...), "-o", "json")...)
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s: %w", r.Kind, err)
	}
	if res.ExitCode != 0 {
		if bytes.Contains(res.Stderr, []byte("(NotFound)")) {
			return nil, nil
		}
		return nil, cmdErr("get "+r.Kind, res)
	}
	var obj map[string]any
	if err := json.Unmarshal(res.Stdout, &obj); err != nil {
		return nil, fmt.Errorf("kubectl get %s: decode output: %w", r.Kind, err)
	}
	if items, ok := obj["items"].([]any); ok && obj["kind"] == "List" {
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	}
	return []map[string]any{obj}, nil
}

// Apply implements Client.
func (k *Kubectl) Apply(ctx context.Context, manifest []byte) error {
	res, err := k.run(ctx, manifest, "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("kubectl apply: %w", err)
	}
	if res.ExitCode != 0 {
		return cmdErr("apply", res)
	}
	return nil
}

// Exec implements Client. A non-zero exit of the remote command is reported in
// the Result, not as an error.
func (k *Kubectl) Exec(ctx context.Context, ns, pod, container string, argv []string) (execx.Result, error) {
	args := []string{"exec", pod}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--")
	res, err := k.run(ctx, nil, append(args, argv...)...)
	if err != nil {
		return res, fmt.Errorf("kubectl exec %s: %w", pod, err)
	}
	return res, nil
}

// CanI implements Client.
func (k *Kubectl) CanI(ctx context.Context, as, verb, resource, sub, ns string) (bool, error) {
	args := []string{"auth", "can-i", verb, resource}
	if sub != "" {
		args = append(args, "--subresource", sub)
	}
	if as != "" {
		args = append(args, "--as", as)
	}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	res, err := k.run(ctx, nil, args...)
	if err != nil {
		return false, fmt.Errorf("kubectl auth can-i: %w", err)
	}
	// can-i exits 1 for "no", so the answer is read from stdout, and anything
	// else is a real failure.
	switch strings.TrimSpace(string(res.Stdout)) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	}
	return false, cmdErr("auth can-i", res)
}

// Wait implements Client.
func (k *Kubectl) Wait(ctx context.Context, r Ref, forCond string, t time.Duration) error {
	args := append([]string{"wait"}, refArgs(r)...)
	args = append(args, "--for="+forCond, fmt.Sprintf("--timeout=%ds", int(t.Seconds())))
	res, err := k.run(ctx, nil, args...)
	if err != nil {
		return fmt.Errorf("kubectl wait: %w", err)
	}
	if res.ExitCode != 0 {
		return cmdErr("wait", res)
	}
	return nil
}
