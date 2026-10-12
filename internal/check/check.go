// Package check defines the check types a scenario can use to grade a cluster.
package check

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

// Env is everything a check may touch.
type Env struct {
	Kube    kube.Client
	Backend backend.Backend
	Cluster string
	// Dir is the materialized scenario directory (cwd for script checks).
	Dir string
	// ScriptEnv holds the KS_* variables passed to scripts.
	ScriptEnv []string
	Runner    execx.Runner
}

// Result is the verdict of one check run.
type Result struct {
	Passed bool
	// Message says why a check failed. It stays short and never reveals the
	// answer.
	Message string
}

// Check is one gradable assertion.
type Check interface {
	// Run returns an error only for infrastructure problems (cluster
	// unreachable, bad probe), never for a wrong answer.
	Run(ctx context.Context, env *Env) (Result, error)
}

// Factory builds a Check from the type-specific JSON of a check spec.
type Factory func(raw json.RawMessage) (Check, error)

// decode strictly decodes raw into v, so typos in scenario files are caught
// by validation instead of silently ignored.
func decode(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func pass() Result { return Result{Passed: true} }

func fail(format string, a ...any) Result {
	return Result{Message: fmt.Sprintf(format, a...)}
}

// sleep waits for d or until ctx ends. A variable so tests need not wait.
var sleep = func(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// RunWithPolicy runs c under the spec's per-attempt timeout and retry policy.
// The first passing attempt wins; otherwise the last attempt's outcome is
// returned.
func RunWithPolicy(ctx context.Context, env *Env, spec scenario.CheckSpec, c Check) (Result, error) {
	attempts := max(spec.Retry.Attempts, 1)
	timeout := spec.Timeout.D()
	if timeout <= 0 {
		timeout = scenario.DefaultCheckTimeout
	}
	var (
		res Result
		err error
	)
	for i := 0; i < attempts; i++ {
		actx, cancel := context.WithTimeout(ctx, timeout)
		res, err = c.Run(actx, env)
		timedOut := actx.Err() != nil && ctx.Err() == nil
		cancel()
		if timedOut && err == nil && !res.Passed {
			err = fmt.Errorf("check timed out after %s", timeout)
		}
		if err == nil && res.Passed {
			return res, nil
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if i < attempts-1 {
			sleep(ctx, spec.Retry.Interval.D())
		}
	}
	return res, err
}

// Validate builds every check of s and reports the problems, so bad check
// definitions fail static validation.
func Validate(s *scenario.Scenario) []error {
	var errs []error
	for i, spec := range s.Spec.Checks {
		if _, err := Build(spec); err != nil {
			errs = append(errs, fmt.Errorf("%s: checks[%d] (%s): %w", s.ID, i, spec.ID, err))
		}
	}
	return errs
}
