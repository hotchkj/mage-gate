// Vision: Test helpers shared by diagnostic filter tests: sentinel table and JSON event assertions.
package gate

import (
	"encoding/json"
	"strings"
	"testing"
)

type sentinelCase struct {
	name     string
	sentinel error
	wantFix  string
	wantHint string
}

var allSentinelCases = []sentinelCase{ //nolint:gochecknoglobals // package-level test fixture; not mutated after init
	{
		"ErrAllPackagesExcluded", ErrAllPackagesExcluded,
		"widen the package scope or reduce exclusions",
		"the current exclude configuration filters out every package, making the check meaningless",
	},
	{
		"ErrLintFailed", ErrLintFailed,
		"address the lint findings in the output below",
		"re-run the lint step in isolation to see full output",
	},
	{
		"ErrFormatFailed", ErrFormatFailed,
		"resolve the formatting issues reported below",
		"run the format step to apply fixes, then re-run lint",
	},
	{
		"ErrCompileFailed", ErrCompileFailed,
		"resolve the compilation error(s) in the output below",
		"check the file and line numbers in the error output",
	},
	{
		"ErrVetFailed", ErrVetFailed,
		"resolve the vet finding(s) in the output below",
		"vet errors indicate code correctness issues, not style",
	},
	{
		"ErrTestFailed", ErrTestFailed,
		"fix the failing test(s) identified in the output below",
		"run the failing test in verbose mode to see detailed output",
	},
	{
		"ErrCoverageFailed", ErrCoverageFailed,
		"add tests to increase coverage to the required minimum",
		"use go tool cover -func on the coverage profile to find uncovered functions",
	},
	{
		"ErrMutationCoverageFailed", ErrMutationCoverageFailed,
		"expand the go test coverage profile (coverpkg) so more mutation points are considered covered",
		"NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages",
	},
	{
		"ErrCrapFailed", ErrCrapFailed,
		"reduce complexity or increase test coverage for the listed functions",
		"CRAP = complexity^2 * (1 - coverage)^3 + complexity; either path reduces the score",
	},
	{
		"ErrDurationFailed", ErrDurationFailed,
		"optimize or split the slow tests listed below",
		"check for sleeps, network calls, expensive setup, or oversized grouped subtests in the slow tests",
	},
	{
		"ErrMutationSitesFailed", ErrMutationSitesFailed,
		"reduce mutation site count by splitting large files",
		"rule of thumb: split by theme, move at least 30% of code out",
	},
	{
		"ErrDeadcodeFailed", ErrDeadcodeFailed,
		"remove or reference the unreachable function(s)",
		"if the function is intentional public API, add it to the deadcode roots build tag",
	},
	{
		"ErrMarkdownLintFailed", ErrMarkdownLintFailed,
		"fix the markdown lint findings in the output below",
		"re-run the markdown lint step in isolation to see full output",
	},
}

func assertNoJSONTestName(t *testing.T, got, wantAbsent string) {
	t.Helper()
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev struct{ Test string }
		if err := json.Unmarshal([]byte(line), &ev); err == nil && ev.Test == wantAbsent {
			t.Fatalf("expected %q to be filtered, but line present: %q", wantAbsent, line)
		}
	}
}

func assertJSONFailEvent(t *testing.T, got, wantTest string) {
	t.Helper()
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev struct {
			Action string
			Test   string
		}
		if err := json.Unmarshal([]byte(line), &ev); err == nil && ev.Action == "fail" && ev.Test == wantTest {
			return
		}
	}
	t.Fatalf("expected fail event for %q to remain, got: %q", wantTest, got)
}
