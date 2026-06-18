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
	testFixMsg  = "expand the go test coverage profile (coverpkg) so more mutation points are considered covered"
	testHintMsg = "NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages"
)

var errMutationCoveragePayloadMissing = errors.New("mutationcoverage payload missing expected format")

func sentinelDiagnosticCases() []struct {
	name     string
	err      error
	wantFix  string
	wantHint string
} {
	return sentinelDiagnosticCaseTable
}

var sentinelDiagnosticCaseTable = []struct {
	name     string
	err      error
	wantFix  string
	wantHint string
}{
	{
		name:     "ErrAllPackagesExcluded",
		err:      fmt.Errorf("%w", ErrAllPackagesExcluded),
		wantFix:  "widen the package scope or reduce exclusions",
		wantHint: "the current exclude configuration filters out every package, making the check meaningless",
	},
	{
		name:     "ErrLintFailed",
		err:      fmt.Errorf("%w", ErrLintFailed),
		wantFix:  "address the lint findings in the output below",
		wantHint: "re-run the lint step in isolation to see full output",
	},
	{
		name:     "ErrFormatFailed",
		err:      fmt.Errorf("%w", ErrFormatFailed),
		wantFix:  "resolve the formatting issues reported below",
		wantHint: "run the format step to apply fixes, then re-run lint",
	},
	{
		name:     "ErrCompileFailed",
		err:      fmt.Errorf("%w", ErrCompileFailed),
		wantFix:  "resolve the compilation error(s) in the output below",
		wantHint: "check the file and line numbers in the error output",
	},
	{
		name:     "ErrVetFailed",
		err:      fmt.Errorf("%w", ErrVetFailed),
		wantFix:  "resolve the vet finding(s) in the output below",
		wantHint: "vet errors indicate code correctness issues, not style",
	},
	{
		name:     "ErrTestFailed",
		err:      fmt.Errorf("%w", ErrTestFailed),
		wantFix:  "fix the failing test(s) identified in the output below",
		wantHint: "run the failing test in verbose mode to see detailed output",
	},
	{
		name:     "ErrCoverageFailed",
		err:      fmt.Errorf("%w", ErrCoverageFailed),
		wantFix:  "add tests to increase coverage to the required minimum",
		wantHint: "use go tool cover -func on the coverage profile to find uncovered functions",
	},
	{
		name:     "ErrMutationCoverageFailed",
		err:      fmt.Errorf("%w", ErrMutationCoverageFailed),
		wantFix:  "expand the go test coverage profile (coverpkg) so more mutation points are considered covered",
		wantHint: "NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages",
	},
	{
		name:     "ErrCrapFailed",
		err:      fmt.Errorf("%w", ErrCrapFailed),
		wantFix:  "reduce complexity or increase test coverage for the listed functions",
		wantHint: "CRAP = complexity^2 * (1 - coverage)^3 + complexity; either path reduces the score",
	},
	{
		name:     "ErrDurationFailed",
		err:      fmt.Errorf("%w", ErrDurationFailed),
		wantFix:  "optimize or split the slow tests listed below",
		wantHint: "check for sleeps, network calls, expensive setup, or oversized grouped subtests in the slow tests",
	},
	{
		name:     "ErrMutationSitesFailed",
		err:      fmt.Errorf("%w", ErrMutationSitesFailed),
		wantFix:  "reduce mutation site count by splitting large files",
		wantHint: "rule of thumb: split by theme, move at least 30% of code out",
	},
	{
		name:     "ErrDeadcodeFailed",
		err:      fmt.Errorf("%w", ErrDeadcodeFailed),
		wantFix:  "remove or reference the unreachable function(s)",
		wantHint: "if the function is intentional public API, add it to the deadcode roots build tag",
	},
	{
		name:     "ErrMarkdownLintFailed",
		err:      fmt.Errorf("%w", ErrMarkdownLintFailed),
		wantFix:  "fix the markdown lint findings in the output below",
		wantHint: "re-run the markdown lint step in isolation to see full output",
	},
	{
		name:     "ErrMutationKillsFailed",
		err:      fmt.Errorf("%w", ErrMutationKillsFailed),
		wantFix:  "improve test coverage or add tests that kill surviving mutations",
		wantHint: "focus on mutations marked as LIVED - these indicate gaps in test coverage",
	},
	{
		name:     "unknown error uses generic fallback",
		err:      errUnclassifiedSentinel,
		wantFix:  "review step-name configuration",
		wantHint: "see step-name output for details",
	},
}

// TestMutationCoverageStructuredDiagnostic verifies that silent mode builds a
// structured DiagnosticError directly from the MutationCoverageResult, with correct
// ERROR/Fix/Hint fields.
func TestMutationCoverageStructuredDiagnostic(t *testing.T) {
	t.Parallel()

	result := buildTestMutationCoverageResult()
	err := buildMutationCoverageDiagnosticFromResult(result)
	if err == nil {
		t.Fatal("expected error")
	}

	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}

	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrMutationCoverageFailed) must be true, got %v", err)
	}

	verifyMutationCoverageDiagnostic(t, de)
}

func buildTestMutationCoverageResult() *gatecheck.MutationCoverageResult {
	return &gatecheck.MutationCoverageResult{
		Summary: struct {
			Percent        float64
			MinPercent     int
			Covered        int
			Total          int
			NotCovered     int
			ThresholdError error
		}{
			Percent:    33.3,
			MinPercent: 67,
			Covered:    2,
			Total:      6,
			NotCovered: 4,
			ThresholdError: fmt.Errorf(
				"%w: mutation coverage 33.3%% below threshold 67%% (2 of 6 mutants covered; 4 not covered by test profile)",
				ErrMutationCoverageFailed,
			),
		},
		WorstFileRows: []gatecheck.MutationCoverageRow{
			{File: "pkg/bad.go", Percent: 0, Total: 3, NotCovered: 3},
			{File: "pkg/ugly.go", Percent: 33.3, Total: 3, NotCovered: 2},
		},
	}
}

func verifyMutationCoverageDiagnostic(t *testing.T, de *DiagnosticError) {
	t.Helper()

	verifyMutationCoverageMessage(t, de.Message())
	verifyMutationCoverageFix(t, de.Fix())
	verifyMutationCoverageHint(t, de.Hint())
	verifyMutationCoverageToolOutput(t, de.ToolOutput())
}

func verifyMutationCoverageMessage(t *testing.T, msg string) {
	t.Helper()
	metrics, parseErr := parseMutationCoverageDiagnosticMessage(msg)
	if parseErr != nil {
		t.Fatalf("%v", parseErr)
	}
	const (
		wantPercent    = 33.3
		wantThreshold  = 67
		wantCovered    = 2
		wantTotal      = 6
		wantNotCovered = 4
	)
	if metrics.percent < wantPercent-0.0001 || metrics.percent > wantPercent+0.0001 {
		t.Fatalf("expected mutation coverage %.1f, got %.1f", wantPercent, metrics.percent)
	}
	if metrics.threshold != wantThreshold {
		t.Fatalf("expected mutation coverage threshold %d, got %d", wantThreshold, metrics.threshold)
	}
	if metrics.covered != wantCovered {
		t.Fatalf("expected covered mutants %d, got %d", wantCovered, metrics.covered)
	}
	if metrics.total != wantTotal {
		t.Fatalf("expected total mutants %d, got %d", wantTotal, metrics.total)
	}
	if metrics.notCovered != wantNotCovered {
		t.Fatalf("expected not covered mutants %d, got %d", wantNotCovered, metrics.notCovered)
	}
}

type mutationCoverageDiagnosticMetrics struct {
	percent    float64
	threshold  int
	covered    int
	total      int
	notCovered int
}

func verifyMutationCoverageFix(t *testing.T, fix string) {
	t.Helper()

	if fix == "" {
		t.Error("expected non-empty Fix")
	}
	if fix != testFixMsg {
		t.Errorf("Fix mismatch:\n  got:  %q\n  want: %q", fix, testFixMsg)
	}
}

func verifyMutationCoverageHint(t *testing.T, hint string) {
	t.Helper()

	if hint == "" {
		t.Error("expected non-empty Hint")
	}
	if hint != testHintMsg {
		t.Errorf("Hint mismatch:\n  got:  %q\n  want: %q", hint, testHintMsg)
	}
}

func verifyMutationCoverageToolOutput(t *testing.T, toolOutput string) {
	t.Helper()

	want := gatecheck.FormatMutationCoverageResultRows(buildTestMutationCoverageResult(), gatecheck.MaxWorstFileRows)
	if toolOutput != want {
		t.Fatalf("ToolOutput mismatch:\n  got:  %q\n  want: %q", toolOutput, want)
	}
}

func parseMutationCoverageDiagnosticMessage(
	message string,
) (metrics mutationCoverageDiagnosticMetrics, parseErr error) {
	const wrappedPrefix = "mutationcoverage failed: "
	const pattern = `^mutation coverage (\d+\.\d)% below threshold (\d+)% \((\d+) of (\d+) mutants covered;` +
		` (\d+) not covered by test profile\)$`
	msg := strings.TrimPrefix(message, wrappedPrefix)
	matches := regexp.MustCompile(pattern).FindStringSubmatch(msg)
	if len(matches) != 6 {
		return metrics, fmt.Errorf("%w: %q", errMutationCoveragePayloadMissing, msg)
	}
	metrics.percent, parseErr = strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage percent %q: %w", matches[1], parseErr)
	}
	metrics.threshold, parseErr = strconv.Atoi(matches[2])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse mutation coverage threshold %q: %w", matches[2], parseErr)
	}
	metrics.covered, parseErr = strconv.Atoi(matches[3])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse covered count %q: %w", matches[3], parseErr)
	}
	metrics.total, parseErr = strconv.Atoi(matches[4])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse total count %q: %w", matches[4], parseErr)
	}
	metrics.notCovered, parseErr = strconv.Atoi(matches[5])
	if parseErr != nil {
		return metrics, fmt.Errorf("parse not covered count %q: %w", matches[5], parseErr)
	}
	return metrics, nil
}
