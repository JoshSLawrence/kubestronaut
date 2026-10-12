package execx

import (
	"context"
	"strings"
	"sync"
)

type rule struct {
	prefix []string
	res    Result
	err    error
	once   bool
	used   bool
}

// Fake records every Cmd and answers from rules matched by argv prefix
// (Name followed by Args). Unmatched commands succeed with empty output.
type Fake struct {
	mu    sync.Mutex
	rules []*rule
	Calls []Cmd
}

// Respond adds a persistent rule. Later rules win over earlier ones.
func (f *Fake) Respond(prefix []string, res Result, err error) {
	f.add(&rule{prefix: prefix, res: res, err: err})
}

// Once adds a rule consumed by its first match. Earlier unconsumed Once rules
// win, so repeated calls can be given a queue of answers.
func (f *Fake) Once(prefix []string, res Result, err error) {
	f.add(&rule{prefix: prefix, res: res, err: err, once: true})
}

func (f *Fake) add(r *rule) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules = append(f.rules, r)
}

func argv(c Cmd) []string { return append([]string{c.Name}, c.Args...) }

func hasPrefix(v, prefix []string) bool {
	if len(v) < len(prefix) {
		return false
	}
	for i, p := range prefix {
		if v[i] != p {
			return false
		}
	}
	return true
}

func (f *Fake) match(c Cmd) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, c)
	v := argv(c)
	for _, r := range f.rules {
		if r.once && !r.used && hasPrefix(v, r.prefix) {
			r.used = true
			return r.res, r.err
		}
	}
	for i := len(f.rules) - 1; i >= 0; i-- {
		r := f.rules[i]
		if !r.once && hasPrefix(v, r.prefix) {
			return r.res, r.err
		}
	}
	return Result{}, nil
}

// Run implements Runner.
func (f *Fake) Run(_ context.Context, c Cmd) (Result, error) { return f.match(c) }

// Interactive implements Runner.
func (f *Fake) Interactive(_ context.Context, c Cmd) error {
	_, err := f.match(c)
	return err
}

// Lines returns each recorded call as one space-joined string.
func (f *Fake) Lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.Calls))
	for i, c := range f.Calls {
		out[i] = strings.Join(argv(c), " ")
	}
	return out
}
