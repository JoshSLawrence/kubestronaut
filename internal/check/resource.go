package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JoshSLawrence/kubestronaut/internal/kube"
)

func init() {
	Register("resourceExists", func(raw json.RawMessage) (Check, error) {
		var c resourceExists
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		return &c, validRef(c.Ref)
	})
	Register("count", func(raw json.RawMessage) (Check, error) {
		var c count
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if c.Eq == nil && c.Gte == nil && c.Lte == nil {
			return nil, errors.New("one of eq, gte or lte is required")
		}
		if c.Eq != nil && (c.Gte != nil || c.Lte != nil) {
			return nil, errors.New("eq cannot be combined with gte or lte")
		}
		return &c, validRef(c.Ref)
	})
}

func validRef(r kube.Ref) error {
	if r.Kind == "" {
		return errors.New("ref.kind is required")
	}
	return nil
}

type resourceExists struct {
	Ref    kube.Ref `json:"ref"`
	Absent bool     `json:"absent"`
}

func (c *resourceExists) Run(ctx context.Context, env *Env) (Result, error) {
	objs, err := env.Kube.Get(ctx, c.Ref)
	if err != nil {
		return Result{}, err
	}
	switch {
	case c.Absent && len(objs) > 0:
		return fail("%s still exists", describe(c.Ref)), nil
	case !c.Absent && len(objs) == 0:
		return fail("%s not found", describe(c.Ref)), nil
	}
	return pass(), nil
}

type count struct {
	Ref kube.Ref `json:"ref"`
	Eq  *int     `json:"eq"`
	Gte *int     `json:"gte"`
	Lte *int     `json:"lte"`
}

func (c *count) Run(ctx context.Context, env *Env) (Result, error) {
	objs, err := env.Kube.Get(ctx, c.Ref)
	if err != nil {
		return Result{}, err
	}
	n := len(objs)
	if (c.Eq != nil && n != *c.Eq) || (c.Gte != nil && n < *c.Gte) || (c.Lte != nil && n > *c.Lte) {
		return fail("%s: found %d object(s), which is not the expected count", describe(c.Ref), n), nil
	}
	return pass(), nil
}

// describe renders a Ref for messages.
func describe(r kube.Ref) string {
	s := r.Kind
	if r.Namespace != "" {
		s += " " + r.Namespace
		if r.Name != "" {
			s += "/" + r.Name
		}
	} else if r.Name != "" {
		s += " " + r.Name
	}
	if r.Selector != "" {
		s += fmt.Sprintf(" (selector %s)", r.Selector)
	}
	return s
}
