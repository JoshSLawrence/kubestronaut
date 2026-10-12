package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/grade"
)

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second:               "45s",
		6*time.Minute + 12*time.Second: "6m12s",
		62 * time.Minute:               "1h02m",
		10 * time.Minute:               "10m",
		2 * time.Hour:                  "2h",
	}
	for d, want := range cases {
		if got := Duration(d); got != want {
			t.Errorf("%v: got %s want %s", d, got, want)
		}
	}
}

func TestGradeText(t *testing.T) {
	r := grade.Report{
		Checks: []grade.CheckOutcome{
			{ID: "a", Description: "first", Weight: 2, Passed: true},
			{ID: "bbb", Description: "second", Weight: 3, Message: "nope"},
			{ID: "c", Description: "third", Weight: 1, Err: "api down"},
		},
		Earned: 2, Possible: 6, Percent: 33.3, PassPercent: 100,
		HintsUsed: 1, SolutionViewed: true, Elapsed: 90 * time.Second,
	}
	var b bytes.Buffer
	GradeText(&b, r, false)
	out := b.String()
	for _, want := range []string{
		"[PASS] 2/2  a    first", "[FAIL] 0/3  bbb  second", "         nope",
		"error: api down", "Score: 2/6 (33%)  -- not passed (needs 100%)",
		"elapsed 1m30s", "hints used: 1", "solution viewed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	b.Reset()
	r.Passed = true
	GradeText(&b, r, true)
	if !strings.Contains(b.String(), "PASSED") || !strings.Contains(b.String(), "(0s)") {
		t.Errorf("verbose/pass output:\n%s", b.String())
	}
}

func TestStepWarnJSON(t *testing.T) {
	var b bytes.Buffer
	Step(&b, "hello %d", 1)
	Warn(&b, "careful")
	if err := JSON(&b, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "==> hello 1\nwarning: careful\n{\n  \"a\": 1\n}") {
		t.Fatalf("got %q", b.String())
	}
}
