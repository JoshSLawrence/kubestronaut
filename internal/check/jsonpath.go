package check

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"k8s.io/client-go/util/jsonpath"

	"github.com/JoshSLawrence/kubestronaut/internal/kube"
)

func init() {
	Register("jsonPath", func(raw json.RawMessage) (Check, error) {
		var c jsonPath
		if err := decode(raw, &c); err != nil {
			return nil, err
		}
		if err := validRef(c.Ref); err != nil {
			return nil, err
		}
		switch c.Match {
		case "", "all", "any":
		default:
			return nil, fmt.Errorf("match must be all or any, got %q", c.Match)
		}
		if c.MinItems < 0 {
			return nil, errors.New("minItems must not be negative")
		}
		p, err := normalizePath(c.Path)
		if err != nil {
			return nil, err
		}
		c.path = p
		m, err := c.compile()
		if err != nil {
			return nil, err
		}
		c.matcher = m
		return &c, nil
	})
}

// scalar reads a string, number or bool from YAML as its string form, so
// `equals: 3` and `equals: "3"` behave the same.
type scalar string

func (s *scalar) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch t := v.(type) {
	case string:
		*s = scalar(t)
	case float64:
		*s = scalar(strconv.FormatFloat(t, 'f', -1, 64))
	case bool:
		*s = scalar(strconv.FormatBool(t))
	default:
		return fmt.Errorf("expected a string, number or bool, got %s", b)
	}
	return nil
}

// Matchers are the value assertions of a jsonPath check; exactly one is set.
type Matchers struct {
	Equals    *scalar  `json:"equals"`
	NotEquals *scalar  `json:"notEquals"`
	Matches   *string  `json:"matches"`
	Contains  *string  `json:"contains"`
	OneOf     []scalar `json:"oneOf"`
	Gte       *float64 `json:"gte"`
	Lte       *float64 `json:"lte"`
	Empty     *bool    `json:"empty"`
	NonEmpty  *bool    `json:"nonEmpty"`
}

type matcher func(val string) (ok bool, msg string)

func (m Matchers) compile() (matcher, error) {
	n := 0
	for _, set := range []bool{
		m.Equals != nil, m.NotEquals != nil, m.Matches != nil, m.Contains != nil,
		m.OneOf != nil, m.Gte != nil, m.Lte != nil, m.Empty != nil, m.NonEmpty != nil,
	} {
		if set {
			n++
		}
	}
	// gte and lte may be combined to express a range.
	if m.Gte != nil && m.Lte != nil && n == 2 {
		n = 1
	}
	if n != 1 {
		return nil, errors.New("exactly one matcher is required: equals, notEquals, matches, contains, oneOf, gte, lte, empty or nonEmpty")
	}
	switch {
	case m.Equals != nil:
		want := string(*m.Equals)
		return func(v string) (bool, string) { return v == want, fmt.Sprintf("got %q", v) }, nil
	case m.NotEquals != nil:
		bad := string(*m.NotEquals)
		return func(v string) (bool, string) { return v != bad, fmt.Sprintf("got %q", v) }, nil
	case m.Matches != nil:
		re, err := regexp.Compile(*m.Matches)
		if err != nil {
			return nil, fmt.Errorf("matches: %w", err)
		}
		return func(v string) (bool, string) { return re.MatchString(v), fmt.Sprintf("got %q", v) }, nil
	case m.Contains != nil:
		sub := *m.Contains
		return func(v string) (bool, string) { return strings.Contains(v, sub), fmt.Sprintf("got %q", v) }, nil
	case m.OneOf != nil:
		if len(m.OneOf) == 0 {
			return nil, errors.New("oneOf must not be empty")
		}
		opts := m.OneOf
		return func(v string) (bool, string) {
			return slices.Contains(opts, scalar(v)), fmt.Sprintf("got %q", v)
		}, nil
	case m.Empty != nil:
		if !*m.Empty {
			return nil, errors.New("empty: false is not meaningful; use nonEmpty: true")
		}
		return func(v string) (bool, string) { return v == "", fmt.Sprintf("got %q", v) }, nil
	case m.NonEmpty != nil:
		if !*m.NonEmpty {
			return nil, errors.New("nonEmpty: false is not meaningful; use empty: true")
		}
		return func(v string) (bool, string) { return v != "", "value is empty or missing" }, nil
	}
	gte, lte := m.Gte, m.Lte
	return func(v string) (bool, string) {
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return false, fmt.Sprintf("value %q is not numeric", v)
		}
		return (gte == nil || f >= *gte) && (lte == nil || f <= *lte), fmt.Sprintf("got %s", v)
	}, nil
}

type jsonPath struct {
	Ref      kube.Ref `json:"ref"`
	Path     string   `json:"path"`
	Match    string   `json:"match"`
	MinItems int      `json:"minItems"`
	Matchers

	path    string
	matcher matcher
}

// normalizePath accepts a path with or without braces and verifies it parses.
func normalizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path is required")
	}
	if !strings.HasPrefix(p, "{") {
		p = "{" + p + "}"
	}
	if err := jsonpath.New("check").Parse(p); err != nil {
		return "", fmt.Errorf("path %q: %w", p, err)
	}
	return p, nil
}

// evalPath returns the kubectl-style text of path against obj. Missing keys
// yield an empty string.
func evalPath(path string, obj map[string]any) string {
	jp := jsonpath.New("check").AllowMissingKeys(true)
	if err := jp.Parse(path); err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := jp.Execute(&buf, obj); err != nil {
		return ""
	}
	return buf.String()
}

func (c *jsonPath) Run(ctx context.Context, env *Env) (Result, error) {
	objs, err := env.Kube.Get(ctx, c.Ref)
	if err != nil {
		return Result{}, err
	}
	minItems := c.MinItems
	if minItems == 0 {
		minItems = 1
	}
	if len(objs) < minItems {
		return fail("%s: found %d object(s), need at least %d", describe(c.Ref), len(objs), minItems), nil
	}
	anyOK := false
	for _, o := range objs {
		ok, msg := c.matcher(evalPath(c.path, o))
		if ok {
			anyOK = true
			continue
		}
		if c.Match != "any" {
			return fail("%s: %s", describe(c.Ref), msg), nil
		}
	}
	if c.Match == "any" && !anyOK {
		return fail("%s: no object matched", describe(c.Ref)), nil
	}
	return pass(), nil
}
