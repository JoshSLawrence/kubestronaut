package check

import (
	"context"
	"encoding/json"
	"errors"
)

func init() {
	Register("canI", func(raw json.RawMessage) (Check, error) {
		var c canI
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if c.As == "" || c.Verb == "" || c.Resource == "" {
			return nil, errors.New("as, verb and resource are required")
		}
		return &c, nil
	})
}

type canI struct {
	As          string `json:"as"`
	Verb        string `json:"verb"`
	Resource    string `json:"resource"`
	Subresource string `json:"subresource"`
	Namespace   string `json:"namespace"`
	// Allowed defaults to true.
	Allowed *bool `json:"allowed"`
}

func (c *canI) Run(ctx context.Context, env *Env) (Result, error) {
	got, err := env.Kube.CanI(ctx, c.As, c.Verb, c.Resource, c.Subresource, c.Namespace)
	if err != nil {
		return Result{}, err
	}
	want := c.Allowed == nil || *c.Allowed
	if got != want {
		if want {
			return fail("%s is not allowed to %s %s", c.As, c.Verb, c.Resource), nil
		}
		return fail("%s is allowed to %s %s but should not be", c.As, c.Verb, c.Resource), nil
	}
	return pass(), nil
}
