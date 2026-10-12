package grade

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/JoshSLawrence/kubestronaut/internal/check"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
)

func checks(t *testing.T, yamls ...string) []scenario.CheckSpec {
	t.Helper()
	var out []scenario.CheckSpec
	for _, y := range yamls {
		j, err := yaml.YAMLToJSON([]byte(y))
		if err != nil {
			t.Fatal(err)
		}
		var c scenario.CheckSpec
		if err := json.Unmarshal(j, &c); err != nil {
			t.Fatal(err)
		}
		if c.Weight == 0 {
			c.Weight = 1
		}
		out = append(out, c)
	}
	return out
}

func scen(passPercent int, cs []scenario.CheckSpec) *scenario.Scenario {
	s := &scenario.Scenario{ID: "cka/x"}
	s.Spec.PassPercent = passPercent
	s.Spec.Checks = cs
	return s
}

func exists(id, name string, weight int) string {
	y := "id: " + id + "\ndescription: " + id + "\ntype: resourceExists\nref: {kind: pod, name: " + name + "}\n"
	if weight > 0 {
		y += "weight: " + strconv.Itoa(weight) + "\n"
	}
	return y
}

func env() *check.Env {
	return &check.Env{Kube: &kube.Fake{Objects: []map[string]any{
		{"kind": "Pod", "metadata": map[string]any{"name": "here", "namespace": "default"}},
	}}}
}

func TestScoring(t *testing.T) {
	cs := checks(t, exists("a", "here", 2), exists("b", "gone", 3), exists("c", "here", 0))
	g := &Grader{Env: env()}

	rep := g.Grade(context.Background(), scen(100, cs))
	if rep.Earned != 3 || rep.Possible != 6 || rep.Percent != 50 || rep.Passed {
		t.Fatalf("report: %+v", rep)
	}
	if got := strings.Join(rep.FailedIDs(), ","); got != "b" {
		t.Fatalf("failed: %s", got)
	}
	if rep.Checks[1].Message == "" || rep.Checks[1].Err != "" || rep.Checks[0].Duration < 0 {
		t.Fatalf("outcome: %+v", rep.Checks[1])
	}

	if rep := g.Grade(context.Background(), scen(50, cs)); !rep.Passed {
		t.Fatal("partial credit at the pass mark must pass")
	}
	if rep := g.Grade(context.Background(), scen(100, cs[:1])); !rep.Passed || rep.Percent != 100 {
		t.Fatalf("all pass: %+v", rep)
	}
}

func TestEmptyScenarioDoesNotPass(t *testing.T) {
	rep := (&Grader{Env: env()}).Grade(context.Background(), scen(100, nil))
	if rep.Passed || rep.AllErrored() {
		t.Fatalf("no checks must not pass: %+v", rep)
	}
}

func TestInfraErrors(t *testing.T) {
	cs := checks(t, exists("a", "here", 0), exists("b", "here", 0),
		"id: bad\ndescription: bad\ntype: nope\n")
	k := &kube.Fake{GetErr: kube.ErrFake()}
	rep := (&Grader{Env: &check.Env{Kube: k}}).Grade(context.Background(), scen(100, cs))
	if !rep.AllErrored() || rep.Earned != 0 || rep.Passed {
		t.Fatalf("report: %+v", rep)
	}
	if !strings.Contains(rep.Checks[2].Err, "unknown check type") {
		t.Fatalf("build error must be reported: %q", rep.Checks[2].Err)
	}

	// One healthy check means the grade is still meaningful.
	k.GetErr = nil
	rep = (&Grader{Env: &check.Env{Kube: k}}).Grade(context.Background(), scen(100, cs))
	if rep.AllErrored() {
		t.Fatal("not all checks errored")
	}
}

func TestGradeTimeout(t *testing.T) {
	cs := checks(t, exists("a", "here", 0), exists("b", "here", 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := (&Grader{Env: env()}).Grade(ctx, scen(100, cs))
	if !rep.AllErrored() || !strings.Contains(rep.Checks[0].Err, "timed out") {
		t.Fatalf("report: %+v", rep)
	}

	// A nanosecond budget exhausts the run while a check is in flight.
	slow := &kube.Fake{GetErr: nil}
	rep = (&Grader{Env: &check.Env{Kube: slow}, Timeout: time.Nanosecond}).Grade(context.Background(), scen(100, cs))
	if rep.Passed {
		t.Fatalf("must not pass: %+v", rep)
	}
}

func TestPassMarkBoundary(t *testing.T) {
	// 2 of 3 is 66.66...%: below a 67 mark, above a 66 mark.
	cs := checks(t, exists("a", "here", 0), exists("b", "here", 0), exists("c", "gone", 0))
	g := &Grader{Env: env()}
	if rep := g.Grade(context.Background(), scen(67, cs)); rep.Passed {
		t.Fatalf("66.67%% must not pass a 67%% mark: %+v", rep)
	}
	if rep := g.Grade(context.Background(), scen(66, cs)); !rep.Passed {
		t.Fatalf("66.67%% must pass a 66%% mark: %+v", rep)
	}
	// Exactly on the mark: 1 of 4 against 25.
	four := checks(t, exists("a", "here", 0), exists("b", "gone", 0), exists("c", "gone", 0), exists("d", "gone", 0))
	if rep := g.Grade(context.Background(), scen(25, four)); !rep.Passed {
		t.Fatalf("exactly 25%% must pass: %+v", rep)
	}
}
