// Vision: Silent mutation-kill diagnostics render structured kill-rate data, never scraped text.
package gate

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const (
	testMutationKillsFixMsg  = "improve test coverage or add tests that kill surviving mutations"
	testMutationKillsHintMsg = "focus on mutations marked as LIVED - these indicate gaps in test coverage"
)

// TestMutationKillsStructuredDiagnostic verifies silent mutation-kill-rate failures expose
// ToolOutput from structured per-file stats (not scraped verbose report text).
func TestMutationKillsStructuredDiagnostic(t *testing.T) {
	t.Parallel()

	result := gatecheck.EvaluateMutationKills(testMutationKillsDiagnosticCheck(), 90)
	if result.Passed {
		t.Fatal("expected EvaluateMutationKills threshold failure fixture")
	}
	err := buildMutationKillsDiagnosticFromResult(result)
	if err == nil {
		t.Fatal("expected error")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("errors.Is(err, ErrMutationKillsFailed) must be true, got %v", err)
	}
	verifyMutationKillsSilentDiagnosticShape(t, de)
}

func TestMutationKillsStructuredDiagnosticTruncatesToolOutput(t *testing.T) {
	t.Parallel()

	check := &gatecheck.MutationKillsCheck{
		TotalKilled: 1, TotalLived: 1, KillRatePercent: 50,
	}
	for i := range 200 {
		check.Files = append(check.Files, gatecheck.FileMutationStats{
			File:  fmt.Sprintf("pkg/file_%03d.go", i),
			Lived: 1,
		})
	}
	err := buildMutationKillsDiagnosticFromResult(gatecheck.EvaluateMutationKills(check, 90))
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}
	wantToolOutput := truncateToolOutput(gatecheck.FormatMutationKillsDetailSections(check))
	if de.ToolOutput() != wantToolOutput {
		t.Fatalf("ToolOutput mismatch:\n  got:  %q\n  want: %q", de.ToolOutput(), wantToolOutput)
	}
	verifyMutationKillsMessage(t, de.Message(), 50.0, 90, "truncation fixture")
	verifyMutationKillsFixHint(t, de)
}

func testMutationKillsDiagnosticCheck() *gatecheck.MutationKillsCheck {
	return &gatecheck.MutationKillsCheck{
		TotalKilled: 8, TotalLived: 7, KillRatePercent: 53.33,
		TotalTimedOut: 1, TotalNotCovered: 0,
		Files: []gatecheck.FileMutationStats{
			{File: "pkg/b.go", Lived: 5},
			{File: "pkg/a.go", Lived: 2},
			{File: "pkg/t.go", TimedOut: 1, TimedOutDetails: []string{"CONDITIONALS_NEGATION line 3 col 9"}},
		},
	}
}

func verifyMutationKillsSilentDiagnosticShape(tb testing.TB, de *DiagnosticError) {
	tb.Helper()
	verifyMutationKillsFixHint(tb, de)
	verifyMutationKillsMessage(tb, de.Message(), 53.3, 90, "diagnostic from fixture")
	verifyMutationKillsToolOutput(tb, de.ToolOutput(), testMutationKillsDiagnosticCheck())
}

func verifyMutationKillsFixHint(tb testing.TB, de *DiagnosticError) {
	tb.Helper()
	if de.Fix() != testMutationKillsFixMsg {
		tb.Errorf("Fix got %q want %q", de.Fix(), testMutationKillsFixMsg)
	}
	if de.Hint() != testMutationKillsHintMsg {
		tb.Errorf("Hint got %q want %q", de.Hint(), testMutationKillsHintMsg)
	}
}

func verifyMutationKillsMessage(tb testing.TB, msg string, wantPercent float64, wantThreshold int, source string) {
	tb.Helper()
	const wrappedPrefix = "mutationkills failed: "
	msg = strings.TrimPrefix(msg, wrappedPrefix)
	const pattern = `^kill rate (\d+(?:\.\d+)?)% below threshold (\d+)%$`
	matches := regexp.MustCompile(pattern).FindStringSubmatch(msg)
	if len(matches) != 3 {
		tb.Fatalf("%s: expected kill-rate threshold payload, got %q", source, msg)
	}
	gotPercent, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		tb.Fatalf("%s: parse kill rate %q: %v", source, matches[1], parseErr)
	}
	gotThreshold, parseErr := strconv.Atoi(matches[2])
	if parseErr != nil {
		tb.Fatalf("%s: parse kill threshold %q: %v", source, matches[2], parseErr)
	}
	if gotPercent < wantPercent-0.0001 || gotPercent > wantPercent+0.0001 {
		tb.Fatalf("%s: expected kill rate %.1f, got %.1f", source, wantPercent, gotPercent)
	}
	if gotThreshold != wantThreshold {
		tb.Fatalf("%s: expected kill threshold %d, got %d", source, wantThreshold, gotThreshold)
	}
	if gotPercent >= float64(wantThreshold) {
		tb.Fatalf("%s: expected failure percentage below threshold, got %.1f >= %d", source, gotPercent, wantThreshold)
	}
}

func verifyMutationKillsToolOutput(tb testing.TB, toolOutput string, check *gatecheck.MutationKillsCheck) {
	tb.Helper()
	if toolOutput == "" {
		tb.Fatal("expected structured ToolOutput with survivor/timeouts sections")
	}
	want := gatecheck.FormatMutationKillsDetailSections(check)
	if toolOutput != want {
		tb.Fatalf("ToolOutput mismatch:\n  got:  %q\n  want: %q", toolOutput, want)
	}
}
