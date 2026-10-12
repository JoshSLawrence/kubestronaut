// Package grade runs a scenario's checks and scores the outcome.
package grade

import (
	"context"
	"errors"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/check"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

// DefaultTimeout bounds a whole grade run.
const DefaultTimeout = 5 * time.Minute

// CheckOutcome is the result of one check.
type CheckOutcome struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Weight      int           `json:"weight"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message,omitempty"`
	Err         string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// Report is the scored result of grading one scenario.
type Report struct {
	Scenario       string         `json:"scenario"`
	Checks         []CheckOutcome `json:"checks"`
	Earned         int            `json:"earned"`
	Possible       int            `json:"possible"`
	Percent        float64        `json:"percent"`
	PassPercent    int            `json:"passPercent"`
	Passed         bool           `json:"passed"`
	HintsUsed      int            `json:"hintsUsed"`
	SolutionViewed bool           `json:"solutionViewed"`
	Elapsed        time.Duration  `json:"elapsed"`
}

// AllErrored reports whether every check hit an infrastructure error, which
// means the grade says nothing about the learner's work.
func (r Report) AllErrored() bool {
	if len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if c.Err == "" {
			return false
		}
	}
	return true
}

// Grader runs checks against one cluster.
type Grader struct {
	Env *check.Env
	// Timeout bounds the whole run; zero means DefaultTimeout.
	Timeout time.Duration
}

// Grade runs every check of s in order. Infrastructure errors count as
// failed checks and are recorded in CheckOutcome.Err.
func (g *Grader) Grade(ctx context.Context, s *scenario.Scenario) Report {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rep := Report{Scenario: s.ID, PassPercent: s.Spec.PassPercent}
	for _, spec := range s.Spec.Checks {
		out := CheckOutcome{ID: spec.ID, Description: spec.Description, Weight: spec.Weight}
		start := time.Now()
		switch {
		case ctx.Err() != nil:
			out.Err = "grade timed out before this check ran"
		default:
			c, err := check.Build(spec)
			if err != nil {
				out.Err = err.Error()
				break
			}
			res, err := check.RunWithPolicy(ctx, g.Env, spec, c)
			switch {
			case errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil:
				out.Err = "grade timed out"
			case err != nil:
				out.Err = err.Error()
			default:
				out.Passed, out.Message = res.Passed, res.Message
			}
		}
		out.Duration = time.Since(start)
		rep.Possible += out.Weight
		if out.Passed {
			rep.Earned += out.Weight
		}
		rep.Checks = append(rep.Checks, out)
	}
	if rep.Possible > 0 {
		rep.Percent = 100 * float64(rep.Earned) / float64(rep.Possible)
	}
	// Integer arithmetic: a float comparison could misjudge a score that sits
	// exactly on the pass mark.
	rep.Passed = rep.Possible > 0 && rep.Earned*100 >= rep.PassPercent*rep.Possible
	return rep
}

// FailedIDs lists the checks that did not pass.
func (r Report) FailedIDs() []string {
	var ids []string
	for _, c := range r.Checks {
		if !c.Passed {
			ids = append(ids, c.ID)
		}
	}
	return ids
}
