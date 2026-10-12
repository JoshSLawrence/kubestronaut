package check

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds a check type. It is called from init() in each check file and
// panics on a duplicate because that is a programming error.
func Register(typ string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[typ]; dup {
		panic("check: duplicate type " + typ)
	}
	registry[typ] = f
}

// Types lists the registered check types.
func Types() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Build creates the Check described by spec.
func Build(spec scenario.CheckSpec) (Check, error) {
	regMu.RLock()
	f, ok := registry[spec.Type]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown check type %q: use one of %v", spec.Type, Types())
	}
	// The shared fields were already decoded into spec.CheckBase; strip them
	// so each type can decode its own fields strictly.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(spec.Raw, &fields); err != nil {
		return nil, fmt.Errorf("check %q: %w", spec.ID, err)
	}
	for _, k := range scenario.BaseKeys {
		delete(fields, k)
	}
	rest, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	c, err := f(rest)
	if err != nil {
		return nil, fmt.Errorf("type %s: %w", spec.Type, err)
	}
	return c, nil
}
