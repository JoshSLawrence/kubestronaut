// Package validate proves that scenarios are well formed (static) and
// solvable (end to end).
package validate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/check"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/grade"
	"github.com/JoshSLawrence/kubestronaut/internal/lab"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/session"
	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

// DefaultE2ETimeout bounds one scenario when validation.timeout is unset.
const DefaultE2ETimeout = 20 * time.Minute

// Static validates the scenarios named by ids (all if empty) and returns every
// problem, including catalog load problems.
func Static(cat *scenario.Catalog, ids []string) []error {
	if len(ids) == 0 {
		errs := append([]error(nil), cat.Problems...)
		for _, s := range cat.Scenarios {
			errs = append(errs, staticOne(cat, s)...)
		}
		return errs
	}
	var errs []error
	for _, id := range ids {
		s, err := cat.Find(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, staticOne(cat, s)...)
	}
	return errs
}

func staticOne(cat *scenario.Catalog, s *scenario.Scenario) []error {
	errs := s.Validate(cat.Packs[s.Pack])
	return append(errs, check.Validate(s)...)
}

// Options configure an end-to-end run.
type Options struct {
	Catalog *scenario.Catalog
	Backend backend.Backend
	Runner  execx.Runner
	Binary  string
	// HomeParent is the state dir of the real install; runs use a private
	// subdirectory so they never touch a learner's session.
	HomeParent string
	Out        io.Writer
	Verbose    bool
	// Keep leaves the cluster running for debugging.
	Keep       bool
	K8sVersion string
	NodeImage  string
	Now        func() time.Time
}

// StepResult is one phase of an end-to-end run.
type StepResult struct {
	Name     string
	Duration time.Duration
}

// Result is the outcome of validating one scenario end to end.
type Result struct {
	Scenario string
	Steps    []StepResult
	Err      error
	Duration time.Duration
}

// OK reports success.
func (r Result) OK() bool { return r.Err == nil }

// E2E starts a scenario in an isolated state dir, checks that every check
// fails before the solution (except validation.passBeforeSolution), runs the
// solution, and checks that everything passes afterwards.
func E2E(ctx context.Context, o Options, id string) (res Result) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	res.Scenario = id
	begin := now()
	record := func(name string, since time.Time) {
		res.Steps = append(res.Steps, StepResult{Name: name, Duration: now().Sub(since)})
	}
	defer func() { res.Duration = now().Sub(begin) }()
	fail := func(format string, a ...any) Result {
		res.Err = fmt.Errorf(format, a...)
		return res
	}

	s, err := o.Catalog.Find(id)
	if err != nil {
		return fail("%v", err)
	}
	res.Scenario = s.ID
	if errs := staticOne(o.Catalog, s); len(errs) > 0 {
		return fail("static validation failed: %v", errors.Join(errs...))
	}
	if existing, err := o.Backend.List(ctx); err != nil {
		return fail("list clusters: %v", err)
	} else if len(existing) > 0 {
		return fail("clusters already exist (%s): run `kubestronaut stop --all` first so the run starts clean", strings.Join(existing, ", "))
	}

	timeout := s.Spec.Validation.Timeout.D()
	if timeout <= 0 {
		timeout = DefaultE2ETimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := os.MkdirAll(o.HomeParent, 0o700); err != nil {
		return fail("%v", err)
	}
	home, err := os.MkdirTemp(o.HomeParent, "validate-")
	if err != nil {
		return fail("%v", err)
	}
	store := &session.Store{Root: home}
	l := lab.New(o.Catalog, o.Backend, store, o.Runner, o.Binary, o.Out)
	l.Verbose, l.Now = o.Verbose, now

	stopped := false
	defer func() {
		if o.Keep {
			ui.Warn(o.Out, "kept cluster and state at %s (stop it with KUBESTRONAUT_HOME=%s kubestronaut stop)", home, home)
			return
		}
		sctx, scancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer scancel()
		if !stopped {
			if err := l.Stop(sctx, false); err != nil && !errors.Is(err, session.ErrNoSession) && res.Err == nil {
				res.Err = fmt.Errorf("cleanup: %w", err)
			}
		}
		_ = os.RemoveAll(home)
	}()

	t := now()
	if err := l.Start(ctx, s.ID, lab.StartOptions{K8sVersion: o.K8sVersion, NodeImage: o.NodeImage}); err != nil {
		return fail("start: %v", err)
	}
	record("start", t)

	t = now()
	base, err := l.Grade(ctx)
	if err != nil {
		return fail("baseline grade: %v", err)
	}
	if err := checkBaseline(s, base); err != nil {
		return fail("baseline: %v", err)
	}
	record("baseline grade (fails as expected)", t)

	t = now()
	if err := l.RunSolution(ctx); err != nil {
		return fail("solution: %v", err)
	}
	record("solution", t)

	t = now()
	final, err := l.Grade(ctx)
	if err != nil {
		return fail("final grade: %v", err)
	}
	if bad := notPassing(final); len(bad) > 0 {
		var b strings.Builder
		ui.GradeText(&b, final, false)
		return fail("after the solution %d check(s) still fail (%s):\n%s", len(bad), strings.Join(bad, ", "), b.String())
	}
	record("final grade (100%)", t)

	t = now()
	if !o.Keep {
		if err := l.Stop(context.WithoutCancel(ctx), false); err != nil {
			return fail("stop: %v", err)
		}
		stopped = true
		record("stop", t)
	}
	return res
}

// checkBaseline requires every check to fail cleanly before the solution,
// except those listed in passBeforeSolution, which may pass. An infra error
// proves nothing, so it fails the run too.
func checkBaseline(s *scenario.Scenario, r grade.Report) error {
	allowed := s.Spec.Validation.PassBeforeSolution
	var problems []string
	for _, c := range r.Checks {
		switch {
		case c.Err != "":
			problems = append(problems, fmt.Sprintf("%s errored: %s", c.ID, c.Err))
		case c.Passed && !slices.Contains(allowed, c.ID):
			problems = append(problems, fmt.Sprintf("%s already passes before the solution (list it in validation.passBeforeSolution if intended)", c.ID))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func notPassing(r grade.Report) []string {
	var ids []string
	for _, c := range r.Checks {
		if !c.Passed {
			ids = append(ids, c.ID)
		}
	}
	if len(r.Checks) == 0 {
		ids = append(ids, "(no checks)")
	}
	return ids
}

// Print writes a one-line summary per step and the verdict.
func Print(w io.Writer, r Result) {
	for _, s := range r.Steps {
		_, _ = fmt.Fprintf(w, "    %-40s %s\n", s.Name, ui.Duration(s.Duration))
	}
	if r.OK() {
		_, _ = fmt.Fprintf(w, "PASS  %s  (%s)\n", r.Scenario, ui.Duration(r.Duration))
		return
	}
	_, _ = fmt.Fprintf(w, "FAIL  %s  (%s)\n      %v\n", r.Scenario, ui.Duration(r.Duration), r.Err)
}

// HomeParent returns where isolated validation state goes.
func HomeParent(root string) string { return filepath.Join(root, "validate") }
