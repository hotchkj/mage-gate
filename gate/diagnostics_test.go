package gate

// Vision: DiagnosticError stability: field ordering, wrapping chains, and silent-display formatting invariants.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

var (
	errUnclassifiedSentinel = errors.New("some unclassified error")
	errGenericStepFailure   = errors.New("something went wrong")
)

func TestDiagnosticFormatCreatesNewDiagnostic(t *testing.T) {
	t.Parallel()

	simpleErr := newValidationError("simple-label", "simple error", ErrInvalidOption)

	wrappedErr := cmdrunner.WrapDiagnostic("simple-label", simpleErr)

	var de *DiagnosticError
	if !errors.As(wrappedErr, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", wrappedErr)
	}
	if de.Name() != "simple-label" {
		t.Errorf("expected name simple-label, got %q", de.Name())
	}
	// Verify structured ERROR/Fix/Hint fields and agent-facing Error() layout.
	if de.Fix() == "" {
		t.Errorf("expected non-empty Fix")
	}
	if de.Hint() == "" {
		t.Errorf("expected non-empty Hint")
	}
	wantErr := cmdtest.ExpectedDiagnosticErrorString(de.Name(), de.Message(), de.Fix(), de.Hint(), de.ToolOutput())
	if wrappedErr.Error() != wantErr {
		t.Errorf("Error() = %q, want %q", wrappedErr.Error(), wantErr)
	}
	// Verify tool output contains validation error info
	if de.ToolOutput() == "" {
		t.Errorf("expected non-empty tool output from validation error")
	}

	if !errors.Is(wrappedErr, ErrInvalidOption) {
		t.Errorf("expected errors.Is(wrappedErr, ErrInvalidOption), chain: %+v", wrappedErr)
	}
}

func TestValidationErrorMessageWithoutStep(t *testing.T) {
	t.Parallel()
	err := newValidationError("", "message only", ErrInvalidOption)
	if err.Error() != "message only" {
		t.Errorf("Error(): got %q, want %q", err.Error(), "message only")
	}
}

func TestRunDiagnosticFormat(t *testing.T) {
	t.Parallel()

	failInner := cmdtest.NewFakeRunner(
		cmdtest.On("go build", gatetest.Fail(errForcedFailure)),
		cmdtest.On("golangci-lint", gatetest.Fail(errForcedFailure)),
	)
	runner := mustNewDisplayRunner(t, failInner, OutputModeAgent, io.Discard, io.Discard)
	fileOps := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot
	pkgScope := mustNewPackageScope(t, "./...")
	err := Compile(context.Background(), runner, fileOps, root, pkgScope)
	if err == nil {
		t.Fatal("expected compile step to fail")
	}

	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", err)
	}
	if de.Name() != "compile" {
		t.Errorf("expected name compile, got %q", de.Name())
	}
	if de.Message() == "" {
		t.Errorf("expected non-empty message")
	}
	if de.Fix() == "" {
		t.Errorf("expected non-empty fix")
	}
	if de.Hint() == "" {
		t.Errorf("expected non-empty hint")
	}
}

// Truncation honors maxToolOutputBytes and backs up so multibyte UTF-8 runes are never split.
func TestTruncateToolOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantTruncated bool
		wantPrefix    string // if non-empty, result must start with this prefix
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "under limit",
			input: strings.Repeat("a", maxToolOutputBytes-1),
		},
		{
			name:  "at limit",
			input: strings.Repeat("a", maxToolOutputBytes),
		},
		{
			name:          "over limit ascii",
			input:         strings.Repeat("a", maxToolOutputBytes+1),
			wantTruncated: true,
			wantPrefix:    strings.Repeat("a", maxToolOutputBytes),
		},
		{
			// "中" is 3 UTF-8 bytes. Placed at bytes [maxToolOutputBytes-2,
			// maxToolOutputBytes+1), the naive cut at maxToolOutputBytes would
			// split the rune; the implementation must retreat to
			// maxToolOutputBytes-2 to emit valid UTF-8 up to the suffix.
			name:          "rune-safe boundary",
			input:         strings.Repeat("a", maxToolOutputBytes-2) + "中",
			wantTruncated: true,
			wantPrefix:    strings.Repeat("a", maxToolOutputBytes-2),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateToolOutput(tc.input)
			if !tc.wantTruncated {
				if got != tc.input {
					t.Fatalf("expected unchanged output (len=%d), got len=%d", len(tc.input), len(got))
				}
				return
			}
			wantSuffix := fmt.Sprintf("\n... (truncated, %d bytes total — full output in verbose mode)", len(tc.input))
			if tc.wantPrefix == "" {
				t.Fatal("test case with wantTruncated must set wantPrefix")
			}
			want := tc.wantPrefix + wantSuffix
			if got != want {
				t.Errorf("truncated output mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

// Matrix: each step sentinel maps to expected Fix/Hint; unknown errors use generic fallback.
func TestSentinelDiagnosticAllSentinels(t *testing.T) {
	t.Parallel()

	for _, tc := range sentinelDiagnosticCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fix, hint := sentinelDiagnostic("step-name", tc.err)
			if fix != tc.wantFix {
				t.Errorf("fix:\n  got:  %q\n  want: %q", fix, tc.wantFix)
			}
			if hint != tc.wantHint {
				t.Errorf("hint:\n  got:  %q\n  want: %q", hint, tc.wantHint)
			}
		})
	}
}

// ErrAllPackagesExcluded should surface ahead of co-present step failures (clearer fix for users).
func TestStepDiagnosticErrAllPackagesExcludedPrecedence(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("%w: %w", ErrCoverageFailed, ErrAllPackagesExcluded)
	result := stepDiagnostic("coverage", err)

	var de *DiagnosticError
	if !errors.As(result, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", result)
	}
	wantFix := "widen the package scope or reduce exclusions"
	if de.Fix() != wantFix {
		t.Errorf("fix:\n  got:  %q\n  want: %q", de.Fix(), wantFix)
	}
}

// Pass-through must keep Fix/Hint from an existing DiagnosticError when re-wrapped.
func TestStepDiagnosticPassThroughPreservesFixHint(t *testing.T) {
	t.Parallel()

	existing := cmdrunner.NewDiagnosticError(
		"lint", "lint broke", "custom-fix", "custom-hint",
		&cmdrunner.DiagnosticOptions{ToolOutput: "some output", Cause: ErrLintFailed},
	)
	wrapped := fmt.Errorf("outer context: %w", existing)
	result := stepDiagnostic("lint", wrapped)

	var de *DiagnosticError
	if !errors.As(result, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", result)
	}
	if de.Fix() != "custom-fix" {
		t.Errorf("fix not preserved: got %q", de.Fix())
	}
	if de.Hint() != "custom-hint" {
		t.Errorf("hint not preserved: got %q", de.Hint())
	}
	// B1 regression: errors.Is must traverse the full chain including the outer
	// wrapper, because Cause is set to err (not de.Unwrap()).
	if !errors.Is(result, ErrLintFailed) {
		t.Error("errors.Is(result, ErrLintFailed) must be true through pass-through chain")
	}
	if !errors.Is(result, existing) {
		t.Error("errors.Is(result, existing) must be true through pass-through chain")
	}
}

// Pass-through still truncates oversized ToolOutput (same cap as fresh diagnostics).
func TestStepDiagnosticPassThroughCapsToolOutput(t *testing.T) {
	t.Parallel()

	longOutput := strings.Repeat("x", maxToolOutputBytes+500)
	existing := cmdrunner.NewDiagnosticError(
		"lint", "lint broke", "fix", "hint",
		&cmdrunner.DiagnosticOptions{ToolOutput: longOutput},
	)
	result := stepDiagnostic("lint", existing)

	var de *DiagnosticError
	if !errors.As(result, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", result)
	}
	wantOutput := truncateToolOutput(longOutput)
	if de.ToolOutput() != wantOutput {
		t.Errorf("ToolOutput mismatch:\n  got:  %q\n  want: %q", de.ToolOutput(), wantOutput)
	}
}

// Unclassified errors get generic Fix/Hint derived from the step name.
func TestStepDiagnosticGenericFallback(t *testing.T) {
	t.Parallel()

	err := errGenericStepFailure
	result := stepDiagnostic("custom-check", err)

	var de *DiagnosticError
	if !errors.As(result, &de) {
		t.Fatalf("expected *DiagnosticError, got %T", result)
	}
	wantFix := "review custom-check configuration"
	if de.Fix() != wantFix {
		t.Errorf("fix:\n  got:  %q\n  want: %q", de.Fix(), wantFix)
	}
	wantHint := "see custom-check output for details"
	if de.Hint() != wantHint {
		t.Errorf("hint:\n  got:  %q\n  want: %q", de.Hint(), wantHint)
	}
}

func TestSentinelDiagnosticCoversAllStepFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range allSentinelCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fix, hint := sentinelDiagnostic("test-step", fmt.Errorf("wrapped: %w", tc.sentinel))
			if fix != tc.wantFix {
				t.Errorf("fix = %q, want %q", fix, tc.wantFix)
			}
			if hint != tc.wantHint {
				t.Errorf("hint = %q, want %q", hint, tc.wantHint)
			}
		})
	}
}

func TestFallbackToolOutputFilter_Deadcode_KeepsUnreachableLines_DropsBanners(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER deadcode startup",
		"deadcode failed: unreachable functions detected",
		"pkg/dead.go:12:1: unreachable func Dead",
	}, "\n")
	got := filterFallbackToolOutput("deadcode", input)
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER deadcode startup" {
			t.Fatalf("expected banner noise to be filtered, but line present: %q", line)
		}
	}
	if !hasExactLine(got, "deadcode failed: unreachable functions detected") {
		t.Fatalf("expected unreachable signal to remain, got: %q", got)
	}
	if !hasExactLine(got, "pkg/dead.go:12:1: unreachable func Dead") {
		t.Fatalf("expected deadcode location to remain, got: %q", got)
	}
}

func TestFallbackToolOutputFilter_Deadcode_KeepsQualifiedFunctionDetails(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER deadcode startup",
		"deadcode failed: unreachable functions detected",
		"example.com/mod/pkg.unreachableFunctionFixture",
	}, "\n")
	got := filterFallbackToolOutput("deadcode", input)
	if !hasExactLine(got, "example.com/mod/pkg.unreachableFunctionFixture") {
		t.Fatalf("expected qualified deadcode detail line to remain, got: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER deadcode startup" {
			t.Fatalf("expected deadcode noise to be removed, but line present: %q", line)
		}
	}
}

func TestFallbackToolOutputFilter_Markdownlint_KeepsFindings_DropsBanners(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER gomarklint startup",
		"markdownlint failed: findings detected",
		"docs/readme.md:3:1: MD041 first-line-heading",
	}, "\n")
	got := filterFallbackToolOutput("markdownlint", input)
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER gomarklint startup" {
			t.Fatalf("expected banner noise to be filtered, but line present: %q", line)
		}
	}
	if !hasExactLine(got, "markdownlint failed: findings detected") {
		t.Fatalf("expected markdownlint signal to remain, got: %q", got)
	}
	if !hasExactLine(got, "docs/readme.md:3:1: MD041 first-line-heading") {
		t.Fatalf("expected markdownlint location to remain, got: %q", got)
	}
}

func TestFallbackToolOutputFilter_Markdownlint_KeepsHighSignalLines(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER gomarklint startup",
		"error: invalid config",
		"docs/guide.md:10:5: trailing whitespace",
	}, "\n")
	got := filterFallbackToolOutput("markdownlint", input)
	if !hasExactLine(got, "error: invalid config") {
		t.Fatalf("expected high-signal error line to remain, got: %q", got)
	}
	if !hasExactLine(got, "docs/guide.md:10:5: trailing whitespace") {
		t.Fatalf("expected markdown detail line to remain, got: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER gomarklint startup" {
			t.Fatalf("expected markdownlint noise to be removed, but line present: %q", line)
		}
	}
}

func TestFallbackToolOutputFilter_Test_KeepsFailures_DropsPassingJSONEvents(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`{"Action":"pass","Test":"TestNoise_PASS_EVENT_SPAM"}`,
		`{"Action":"run","Test":"TestNoise_PASS_EVENT_SPAM"}`,
		`{"Action":"output","Output":"--- FAIL: TestImportant (0.00s)\n"}`,
		`{"Action":"fail","Test":"TestImportant"}`,
		"test failed: go test: exit status 1",
	}, "\n")
	got := filterFallbackToolOutput("test", input)
	assertNoJSONTestName(t, got, "TestNoise_PASS_EVENT_SPAM")
	if !hasExactLine(got, `{"Action":"output","Output":"--- FAIL: TestImportant (0.00s)\n"}`) {
		t.Fatalf("expected failing test marker to remain, got: %q", got)
	}
	assertJSONFailEvent(t, got, "TestImportant")
}

func TestFallbackToolOutputFilter_Test_KeepsCompileErrorContext(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`{"Action":"pass","Test":"PASS_EVENT_SPAM"}`,
		`# example.com/mod/pkg`,
		`pkg/compile.go:9:2: undefined: missingSymbol`,
		"test failed: go test: exit status 1",
	}, "\n")
	got := filterFallbackToolOutput("test", input)
	if !hasExactLine(got, "# example.com/mod/pkg") {
		t.Fatalf("expected compiler package line to remain, got: %q", got)
	}
	if !hasExactLine(got, "pkg/compile.go:9:2: undefined: missingSymbol") {
		t.Fatalf("expected compiler location to remain, got: %q", got)
	}
	assertNoJSONTestName(t, got, "PASS_EVENT_SPAM")
}

func TestFallbackToolOutputFilter_Crap_KeepsSummaryAndTopN_DropsTail(t *testing.T) {
	t.Parallel()

	lines := []string{
		"crap failed: CRAP check failed: 25 function(s) exceed threshold of 8.0",
		"CRAP check failed: 25 function(s) exceed threshold of 8.0",
	}
	for i := 1; i <= 25; i++ {
		lines = append(lines, fmt.Sprintf("  pkg/file.go:Fn%d - CRAP=%.1f (complexity=10, coverage=0.0%%)", i, float64(i)))
	}
	got := filterFallbackToolOutput("crap", strings.Join(lines, "\n"))
	if !hasExactLine(got, "crap failed: CRAP check failed: 25 function(s) exceed threshold of 8.0") {
		t.Fatalf("expected CRAP summary to remain, got: %q", got)
	}
	unwantedLine := "  pkg/file.go:Fn25 - CRAP=25.0 (complexity=10, coverage=0.0%)"
	for _, line := range strings.Split(got, "\n") {
		if line == unwantedLine {
			t.Fatalf("expected tail offender to be filtered, but line present: %q", line)
		}
	}
	if !hasExactLine(got, "  ... and 5 more offender(s)") {
		t.Fatalf("expected remainder summary, got: %q", got)
	}
}

func TestFallbackToolOutputFilter_MutationKills_StepNameDoesNotInvokeLineScraper(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER mutationkills prelude",
		"Survivors by file (LIVED > 0):",
		"   3  pkg/one.go",
		"Timed out by file (TIMED_OUT > 0):",
		"   1  pkg/two.go",
		"      - CONDITIONALS_NEGATION line 3 col 9",
	}, "\n")
	got := filterFallbackToolOutput("mutationkills", input)
	if got != input {
		t.Fatalf(`mutationkills must use passthrough default (silent kill-rate thresholds use structured ToolOutput; `+
			`this filter is only for scraped stderr text), got:\n%q`, got)
	}
}

func TestFallbackToolOutputFilter_UnknownStepName_ReturnsSignalOnly(t *testing.T) {
	t.Parallel()
	got := filterFallbackToolOutput("custom-step", strings.Join([]string{
		"NOISE_BANNER custom-step prelude",
		"important failure: custom-step failed",
		"  example.go:10:2: unexpected",
		"tail noise",
	}, "\n"))
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER custom-step prelude" || line == "tail noise" {
			t.Fatalf("expected unknown step noise to be filtered: %q", line)
		}
	}
	if !hasExactLine(got, "important failure: custom-step failed") {
		t.Fatalf("expected failure line to remain, got %q", got)
	}
}

func TestFallbackToolOutputFilter_MutationSites_KeepsThresholdAndTopFiles_DropsNoise(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NOISE_BANNER mutationsites prelude",
		"mutationsites failed: mutation sites 150 exceed threshold 100",
		"Top files by mutation sites:",
		"  pkg/large.go: 85 sites",
		"  pkg/medium.go: 45 sites",
	}, "\n")
	got := filterFallbackToolOutput("mutationsites", input)
	for _, line := range strings.Split(got, "\n") {
		if line == "NOISE_BANNER mutationsites prelude" {
			t.Fatalf("expected mutationsites noise to be filtered, but line present: %q", line)
		}
	}
	if !hasExactLine(got, "mutationsites failed: mutation sites 150 exceed threshold 100") {
		t.Fatalf("expected threshold signal to remain, got: %q", got)
	}
	if !hasExactLine(got, "Top files by mutation sites:") {
		t.Fatalf("expected top files header to remain, got: %q", got)
	}
	if !hasExactLine(got, "  pkg/large.go: 85 sites") {
		t.Fatalf("expected top file line to remain, got: %q", got)
	}
	if !hasExactLine(got, "  pkg/medium.go: 45 sites") {
		t.Fatalf("expected top file line to remain, got: %q", got)
	}
}
