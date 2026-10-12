package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JoshSLawrence/kubestronaut/internal/execx"
)

func init() {
	Register("script", func(raw json.RawMessage) (Check, error) {
		var c script
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if c.Path == "" {
			return nil, errors.New("path is required")
		}
		return &c, nil
	})
}

type script struct {
	Path string            `json:"path"`
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
}

func (c *script) Run(ctx context.Context, env *Env) (Result, error) {
	vars := append([]string(nil), env.ScriptEnv...)
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vars = append(vars, k+"="+c.Env[k])
	}
	res, err := env.Runner.Run(ctx, execx.Cmd{
		Name: "bash", Args: append([]string{c.Path}, c.Args...), Env: vars, Dir: env.Dir,
	})
	if err != nil {
		return Result{}, fmt.Errorf("run %s: %w", c.Path, err)
	}
	if res.ExitCode == 0 {
		return pass(), nil
	}
	return fail("%s", lastLine(string(res.Stderr), fmt.Sprintf("script exited %d", res.ExitCode))), nil
}

func lastLine(s, fallback string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return fallback
}
