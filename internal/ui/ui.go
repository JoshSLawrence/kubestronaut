// Package ui renders output for the terminal and for --json.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/grade"
)

// Step prints a progress line in the style of `==> doing something`.
func Step(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, "==> "+format+"\n", a...)
}

// Warn prints a non-fatal problem.
func Warn(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, "warning: "+format+"\n", a...)
}

// Duration formats d for people: 6m12s, 1h02m, 45s.
func Duration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// GradeText prints a score breakdown. With verbose, check durations are shown.
func GradeText(w io.Writer, r grade.Report, verbose bool) {
	idw := 0
	for _, c := range r.Checks {
		idw = max(idw, len(c.ID))
	}
	for _, c := range r.Checks {
		tag, pts := "FAIL", 0
		if c.Passed {
			tag, pts = "PASS", c.Weight
		}
		line := fmt.Sprintf("  [%s] %d/%d  %-*s  %s", tag, pts, c.Weight, idw, c.ID, c.Description)
		if verbose {
			line += fmt.Sprintf("  (%s)", c.Duration.Round(time.Millisecond))
		}
		_, _ = fmt.Fprintln(w, line)
		switch {
		case c.Err != "":
			_, _ = fmt.Fprintf(w, "         error: %s\n", c.Err)
		case !c.Passed && c.Message != "":
			_, _ = fmt.Fprintf(w, "         %s\n", c.Message)
		}
	}
	verdict := fmt.Sprintf("not passed (needs %d%%)", r.PassPercent)
	if r.Passed {
		verdict = "PASSED"
	}
	var extras []string
	if r.Elapsed > 0 {
		extras = append(extras, "elapsed "+Duration(r.Elapsed))
	}
	if r.HintsUsed > 0 {
		extras = append(extras, fmt.Sprintf("hints used: %d", r.HintsUsed))
	}
	if r.SolutionViewed {
		extras = append(extras, "solution viewed")
	}
	tail := ""
	if len(extras) > 0 {
		tail = "  " + strings.Join(extras, "  ")
	}
	_, _ = fmt.Fprintf(w, "  Score: %d/%d (%.0f%%)  -- %s%s\n", r.Earned, r.Possible, r.Percent, verdict, tail)
}

// JSON writes v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
