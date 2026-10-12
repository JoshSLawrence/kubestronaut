package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

func init() {
	Register("connectivity", func(raw json.RawMessage) (Check, error) {
		var c connectivity
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if err := validRef(c.From); err != nil {
			return nil, fmt.Errorf("from: %w", err)
		}
		if c.From.Name == "" {
			return nil, errors.New("from.name is required")
		}
		host, port, err := net.SplitHostPort(c.To)
		if err != nil || host == "" || port == "" {
			return nil, fmt.Errorf("to %q must look like host:port", c.To)
		}
		c.host, c.port = host, port
		switch c.Expect {
		case "allowed", "denied":
		default:
			return nil, fmt.Errorf("expect must be allowed or denied, got %q", c.Expect)
		}
		switch c.Probe {
		case "":
			c.Probe = "agnhost"
		case "agnhost", "nc":
		default:
			return nil, fmt.Errorf("probe must be agnhost or nc, got %q", c.Probe)
		}
		if c.ConnectTimeout == 0 {
			c.ConnectTimeout = scenario.Duration(3 * time.Second)
		}
		return &c, nil
	})
}

type connectivity struct {
	From           kube.Ref          `json:"from"`
	To             string            `json:"to"`
	Expect         string            `json:"expect"`
	Probe          string            `json:"probe"`
	ConnectTimeout scenario.Duration `json:"connectTimeout"`

	host, port string
}

func (c *connectivity) argv() []string {
	secs := max(int(c.ConnectTimeout.D().Seconds()), 1)
	if c.Probe == "nc" {
		return []string{"nc", "-z", "-w", fmt.Sprint(secs), c.host, c.port}
	}
	return []string{"/agnhost", "connect", c.To, fmt.Sprintf("--timeout=%ds", secs)}
}

func (c *connectivity) Run(ctx context.Context, env *Env) (Result, error) {
	// A missing source pod would look like "denied"; make it an infra error.
	pods, err := env.Kube.Get(ctx, c.From)
	if err != nil {
		return Result{}, err
	}
	if len(pods) == 0 {
		return Result{}, fmt.Errorf("source pod %s not found", describe(c.From))
	}
	if c.Probe == "nc" && c.Expect == "denied" && net.ParseIP(c.host) == nil {
		if err := c.checkResolves(ctx, env); err != nil {
			return Result{}, err
		}
	}
	res, err := env.Kube.Exec(ctx, c.From.Namespace, c.From.Name, "", c.argv())
	if err != nil {
		return Result{}, err
	}
	out := string(res.Stdout) + string(res.Stderr)
	if res.ExitCode != 0 && isKubectlFailure(res) {
		return Result{}, fmt.Errorf("could not exec in %s: %s", describe(c.From), strings.TrimSpace(string(res.Stderr)))
	}
	connected := res.ExitCode == 0
	if !connected && c.Probe == "nc" && c.Expect == "denied" && strings.Contains(out, "bad address") {
		return Result{}, fmt.Errorf("host %s did not resolve: %s", c.host, strings.TrimSpace(out))
	}
	if !connected && c.Probe == "agnhost" && c.Expect == "denied" {
		// Only a timeout or a refusal is a network decision. A name that does
		// not resolve, or anything unrecognised, must not read as "denied".
		switch {
		case strings.Contains(out, "DNS"):
			return Result{}, fmt.Errorf("host %s did not resolve: %s", c.host, strings.TrimSpace(out))
		case !strings.Contains(out, "TIMEOUT") && !strings.Contains(out, "REFUSED"):
			return Result{}, fmt.Errorf("unexpected probe output from %s: %q", describe(c.From), strings.TrimSpace(out))
		}
	}
	switch {
	case c.Expect == "allowed" && !connected:
		return fail("%s cannot reach %s", describe(c.From), c.To), nil
	case c.Expect == "denied" && connected:
		return fail("%s can reach %s but should not", describe(c.From), c.To), nil
	}
	return pass(), nil
}

// checkResolves makes sure a failing nc is a network decision and not a name
// that does not resolve. nc cannot tell the two apart reliably, so the name is
// looked up first; images without nslookup skip this and rely on nc's own
// "bad address" message.
func (c *connectivity) checkResolves(ctx context.Context, env *Env) error {
	res, err := env.Kube.Exec(ctx, c.From.Namespace, c.From.Name, "", []string{"nslookup", c.host})
	if err != nil {
		return err
	}
	out := string(res.Stdout) + string(res.Stderr)
	switch {
	case res.ExitCode == 0:
		return nil
	case isKubectlFailure(res):
		return fmt.Errorf("could not exec in %s: %s", describe(c.From), strings.TrimSpace(string(res.Stderr)))
	case res.ExitCode == 127 || strings.Contains(out, "not found"):
		return nil
	}
	return fmt.Errorf("host %s did not resolve: %s", c.host, strings.TrimSpace(out))
}
