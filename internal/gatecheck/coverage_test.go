// Vision: Line/statement coverage totals vs minimum percent, including empty-profile and rounding edges.
package gatecheck

import (
	"errors"
	"fmt"
	"testing"
)

const oneCoveredStatementProfile = `mode: set
github.com/hotchkj/mage-gate/internal/harness/config.go:1.0,2.0 1 1
`

func TestCoverage_Passed(t *testing.T) {
	t.Parallel()
	// Raw coverage.out profile format
	profile := `mode: set
github.com/hotchkj/mage-gate/internal/harness/config.go:1.0,2.0 1 1
github.com/hotchkj/mage-gate/internal/harness/deps.go:1.0,2.0 1 1
`
	result, err := Coverage(profile, 90.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected passed, got failed (coverage=%.1f)", result.TotalCoverage)
	}
	// With 2 stmts covered out of 2 total, coverage should be 100%
	if result.TotalCoverage != 100.0 {
		t.Fatalf("expected 100.0, got %.1f", result.TotalCoverage)
	}
}

func TestCoverage_Failed(t *testing.T) {
	t.Parallel()
	// 1 covered out of 2 = 50%
	profile := `mode: set
github.com/hotchkj/mage-gate/internal/harness/config.go:1.0,2.0 1 1
github.com/hotchkj/mage-gate/internal/harness/deps.go:1.0,2.0 1 0
`
	result, err := Coverage(profile, 90.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if result.Passed {
		t.Fatal("expected failed, got passed")
	}
	if !errors.Is(result.ThresholdError, ErrCoverageFailed) {
		t.Fatalf("expected threshold error to wrap ErrCoverageFailed, got %v", result.ThresholdError)
	}
}

func TestCoverage_WorstFileRowsUseStatementCoverage(t *testing.T) {
	t.Parallel()

	profile := `mode: set
pkg/zero.go:1.0,2.0 4 0
pkg/partial.go:1.0,2.0 1 1
pkg/partial.go:3.0,4.0 3 0
pkg/full.go:1.0,2.0 5 1
`
	result, err := Coverage(profile, 90.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if result.CoveredStatements != 6 || result.TotalStatements != 13 {
		t.Fatalf("statement counts: got %d/%d, want 6/13", result.CoveredStatements, result.TotalStatements)
	}
	if len(result.WorstFileRows) != 2 {
		t.Fatalf("worst rows len = %d, want 2: %#v", len(result.WorstFileRows), result.WorstFileRows)
	}
	assertCoverageRow(t, result.WorstFileRows[0], "pkg/zero.go", 0, 0, 4)
	assertCoverageRow(t, result.WorstFileRows[1], "pkg/partial.go", 25, 1, 4)
}

func TestCoverage_WorstFileRowsOrderEqualPercentByFile(t *testing.T) {
	t.Parallel()

	profile := `mode: set
pkg/b.go:1.0,2.0 1 1
pkg/b.go:3.0,4.0 1 0
pkg/a.go:1.0,2.0 1 1
pkg/a.go:3.0,4.0 1 0
`
	result, err := Coverage(profile, 90.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if len(result.WorstFileRows) != 2 {
		t.Fatalf("worst rows len = %d, want 2: %#v", len(result.WorstFileRows), result.WorstFileRows)
	}
	assertCoverageRow(t, result.WorstFileRows[0], "pkg/a.go", 50, 1, 2)
	assertCoverageRow(t, result.WorstFileRows[1], "pkg/b.go", 50, 1, 2)
}

func TestCoverage_WorstFileRowsAreCapped(t *testing.T) {
	t.Parallel()

	profile := "mode: set\n"
	for i := 0; i < MaxWorstFileRows+1; i++ {
		profile += fmt.Sprintf("pkg/file%02d.go:1.0,2.0 1 0\n", i)
	}
	result, err := Coverage(profile, 90.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if len(result.WorstFileRows) != MaxWorstFileRows {
		t.Fatalf("worst rows len = %d, want %d", len(result.WorstFileRows), MaxWorstFileRows)
	}
	assertCoverageRow(t, result.WorstFileRows[MaxWorstFileRows-1], "pkg/file09.go", 0, 0, 1)
}

func assertCoverageRow(
	t *testing.T,
	row CoverageFileRow,
	wantFile string,
	wantPercent float64,
	wantCovered, wantTotal int,
) {
	t.Helper()
	if row.File != wantFile {
		t.Fatalf("row file = %q, want %q", row.File, wantFile)
	}
	if row.Percent != wantPercent {
		t.Fatalf("row percent = %.1f, want %.1f", row.Percent, wantPercent)
	}
	if row.CoveredStatements != wantCovered || row.TotalStatements != wantTotal {
		t.Fatalf(
			"row statements = %d/%d, want %d/%d",
			row.CoveredStatements,
			row.TotalStatements,
			wantCovered,
			wantTotal,
		)
	}
}

func TestFormatCoverageDiagnosticRows(t *testing.T) {
	t.Parallel()

	result := CoverageResult{
		WorstFileRows: []CoverageFileRow{
			{File: "pkg/zero.go", Percent: 0, CoveredStatements: 0, TotalStatements: 4},
			{File: "pkg/partial.go", Percent: 25, CoveredStatements: 1, TotalStatements: 4},
			{File: "pkg/another.go", Percent: 50, CoveredStatements: 1, TotalStatements: 2},
		},
	}
	got := FormatCoverageDiagnosticRows(&result, 2)
	want := "Worst coverage files:\n" +
		"  0.0%  pkg/zero.go (0/4 statements covered)\n" +
		"  25.0%  pkg/partial.go (1/4 statements covered)\n" +
		"  ... and 1 more files"
	if got != want {
		t.Fatalf("formatted rows:\ngot  %q\nwant %q", got, want)
	}
}

func TestCoverage_FailsWhenTotalBelowSmallPositiveMin(t *testing.T) {
	t.Parallel()
	// 1 covered out of 1000 = 0.1% (need many stmts to get low percentage)
	// 0.1% < 0.5% so should fail
	profile := oneCoveredStatementProfile
	// Add 999 more stmts that are not covered, each with unique line numbers
	for i := 0; i < 999; i++ {
		profile += fmt.Sprintf("github.com/hotchkj/mage-gate/internal/harness/deps.go:%d.0,%d.0 1 0\n", i+1, i+1)
	}
	result, err := Coverage(profile, 0.5)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failed when total is below a small positive minimum, got coverage=%.1f", result.TotalCoverage)
	}
}

func TestCoverage_MinZeroDisablesThreshold(t *testing.T) {
	t.Parallel()
	profile := oneCoveredStatementProfile
	result, err := Coverage(profile, 0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected zero min threshold to pass regardless of coverage")
	}
	if result.TotalCoverage != 100.0 {
		t.Fatalf("expected total 100.0, got %.1f", result.TotalCoverage)
	}
}

func TestCoverage_NegativeMinDisablesThreshold(t *testing.T) {
	t.Parallel()
	profile := `mode: set
github.com/hotchkj/mage-gate/internal/harness/config.go:1.0,2.0 1 1
`
	result, err := Coverage(profile, -0.5)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected negative min to disable threshold like zero")
	}
}

func TestCoverage_TotalEqualsMinPasses(t *testing.T) {
	t.Parallel()
	// 1 covered out of 2 = 50%
	profile := `mode: set
github.com/hotchkj/mage-gate/internal/harness/config.go:1.0,2.0 1 1
github.com/hotchkj/mage-gate/internal/harness/deps.go:1.0,2.0 1 0
`
	result, err := Coverage(profile, 50.0)
	if err != nil {
		t.Fatalf("Coverage() error = %v", err)
	}
	if !result.Passed {
		t.Fatal("expected pass when total equals min (>= boundary)")
	}
}

func TestCoverage_EmptyProfile(t *testing.T) {
	t.Parallel()
	profile := `mode: set
`
	_, err := Coverage(profile, 90.0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errNoCoverageData) {
		t.Fatalf("expected errNoCoverageData, got %v", err)
	}
}

func TestFilterCoverageProfile_NoExcludes_ReturnsSame(t *testing.T) {
	t.Parallel()
	input := "mode: set\npkg/a.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, nil, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	if out != input {
		t.Fatalf("expected unchanged, got %q", out)
	}
}

func TestFilterCoverageProfile_EmptyProfile(t *testing.T) {
	t.Parallel()
	out, err := FilterCoverageProfile("", []string{"exclude"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
}

func TestFilterCoverageProfile_NoMatchingExcludes(t *testing.T) {
	t.Parallel()
	input := "mode: set\npkg/keep.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, []string{"/drop/"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	want := "mode: set\npkg/keep.go:1.1,2.2 3 1\n"
	if out != want {
		t.Fatalf("expected mode and keep line, got %q, want %q", out, want)
	}
}

func TestFilterCoverageProfile_MatchingExcludes(t *testing.T) {
	t.Parallel()
	input := "pkg/drop/x.go:1.1,2.2 3 1\npkg/keep/y.go:3.3,4.4 5 1\n"
	out, err := FilterCoverageProfile(input, []string{"drop"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	wantOut := "pkg/keep/y.go:3.3,4.4 5 1\n"
	if out != wantOut {
		t.Fatalf("expected keep path only, got %q, want %q", out, wantOut)
	}
}

func TestFilterCoverageProfile_ExcludesFeatureOrHarnessFilePaths(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"github.com/x/mod/features/bdd/ctx.go:1.1,2.2 3 0\n" +
		"github.com/x/mod/gatetest/fake.go:1.1,2.2 3 1\n" +
		"github.com/x/mod/gate/opts.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, []string{"features", "gatetest"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	want := "mode: set\ngithub.com/x/mod/gate/opts.go:1.1,2.2 3 1\n"
	if out != want {
		t.Fatalf("filtered profile\ngot  %q\nwant %q", out, want)
	}
}

func TestFilterCoverageProfile_TestFilePatterns(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"github.com/x/mod/gate/keep.go:1.1,2.2 3 1\n" +
		"github.com/x/mod/gate/keep_test.go:1.1,2.2 3 0\n"
	out, err := FilterCoverageProfile(input, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	want := "mode: set\ngithub.com/x/mod/gate/keep.go:1.1,2.2 3 1\n"
	if out != want {
		t.Fatalf("filtered profile\ngot  %q\nwant %q", out, want)
	}
}

func TestFilterCoverageProfile_SpacePath_ExcludedBySegment_WithoutNestedPrefix(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"github.com/x/mod/pkg with spaces/keep.go:1.1,2.2 3 1\n" +
		"github.com/x/mod/pkg/other.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, []string{"pkg with spaces"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	want := "mode: set\ngithub.com/x/mod/pkg/other.go:1.1,2.2 3 1\n"
	if out != want {
		t.Fatalf("filtered profile with space segment\ngot  %q\nwant %q", out, want)
	}
}

func TestFilterCoverageProfile_SpacePath_DeterministicWithPatterns(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"/tmp/repo/pkg with spaces/keep_test.go:1.1,2.2 3 0\n" +
		"/tmp/repo/pkg with spaces/keep.go:1.1,2.2 3 1\n" +
		"/tmp/repo/pkg/excluded/target.go:1.1,2.2 3 1\n"
	want := "mode: set\n/tmp/repo/pkg with spaces/keep.go:1.1,2.2 3 1\n"

	first, err := FilterCoverageProfile(input, []string{"pkg/excluded"}, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	second, err := FilterCoverageProfile(input, []string{"pkg/excluded"}, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}

	if first != want {
		t.Fatalf("filtered profile with spaces\ngot  %q\nwant %q", first, want)
	}
	if first != second {
		t.Fatalf("non-deterministic filter output\ngot  %q\nother %q", first, second)
	}
}

func TestFilterCoverageProfile_LineWithoutColonIncluded(t *testing.T) {
	t.Parallel()
	input := "orphan-line\npkg/drop/x.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, []string{"drop"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	wantOrphan := "orphan-line\n"
	if out != wantOrphan {
		t.Fatalf("expected orphan line only, got %q, want %q", out, wantOrphan)
	}
}

func TestFilterCoverageProfile_SpacePath_KeptWhenNoFilters(t *testing.T) {
	t.Parallel()
	input := "mode: set\npkg/with space/file one.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, nil, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	if out != input {
		t.Fatalf("expected unchanged profile, got %q, want %q", out, input)
	}
}

func TestFilterCoverageProfile_SpacePath_ExcludedBySegment(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"pkg/with space/file one.go:1.1,2.2 3 1\n" +
		"pkg/clean/file_two.go:1.1,2.2 3 1\n"
	out, err := FilterCoverageProfile(input, []string{"with space"}, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	want := "mode: set\npkg/clean/file_two.go:1.1,2.2 3 1\n"
	if out != want {
		t.Fatalf("expected only clean path, got %q, want %q", out, want)
	}
}

func TestFilterCoverageProfile_SpacePath_DeterministicWithTestPattern_LocalPaths(t *testing.T) {
	t.Parallel()
	input := "mode: set\n" +
		"pkg/with space/keep.go:1.1,2.2 3 1\n" +
		"pkg/with space/keep_test.go:1.1,2.2 3 0\n"
	first, err := FilterCoverageProfile(input, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	second, err := FilterCoverageProfile(input, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("FilterCoverageProfile() second pass error = %v", err)
	}
	want := "mode: set\npkg/with space/keep.go:1.1,2.2 3 1\n"
	if first != want || second != want || first != second {
		t.Fatalf("first=%q second=%q want=%q", first, second, want)
	}
}

func TestCoverProfileFilePathFromToken_PreservesDriveAbsolutePath(t *testing.T) {
	t.Parallel()
	got := coverProfileFilePath(`C:/repo/pkg/file.go:10:1`)
	if got != `C:/repo/pkg/file.go` {
		t.Fatalf("coverProfileFilePath() = %q, want %q", got, `C:/repo/pkg/file.go`)
	}
}

func TestCoverProfileFilePathFromToken_PreservesUnixAbsolutePath(t *testing.T) {
	t.Parallel()
	got := coverProfileFilePath(`/tmp/repo/pkg/file.go:10:1`)
	if got != `/tmp/repo/pkg/file.go` {
		t.Fatalf("coverProfileFilePath() = %q, want %q", got, `/tmp/repo/pkg/file.go`)
	}
}

func TestCoverProfileFilePathFromToken_MalformedPathLikePrefixRemainsIntact(t *testing.T) {
	t.Parallel()
	got := coverProfileFilePath(`part:with:colon`)
	if got != `part:with:colon` {
		t.Fatalf("coverProfileFilePath() = %q, want %q", got, `part:with:colon`)
	}
}

func TestKeepCoverageLine_TracksWindowsAndUnixPaths(t *testing.T) {
	t.Parallel()
	input := `mode: set
/tmp/repo/pkg/keep.go:1.1,2.2 3 1
C:/repo/pkg/keep.go:1.1,2.2 3 1
`
	got, err := FilterCoverageProfile(input, nil, nil)
	if err != nil {
		t.Fatalf("FilterCoverageProfile() error = %v", err)
	}
	if got != input {
		t.Fatalf("expected unchanged profile, got %q, want %q", got, input)
	}
}

func TestIsLineOrCol_RejectsMultipleDots(t *testing.T) {
	t.Parallel()

	if got := isLineOrCol("1.2.3"); got {
		t.Fatalf("isLineOrCol() = %v, want false", got)
	}
}

func TestIsLineOrCol_AcceptsIntegerAndDecimalComponents(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"12",
		"12.34",
	}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			if got := isLineOrCol(tc); !got {
				t.Fatalf("isLineOrCol(%q) = %v, want true", tc, got)
			}
		})
	}
}

func TestIsLineOrCol_RejectsNonNumericInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []string{"", "a", "1.a", "1..2", "1.2x"} {
		t.Run(tc, func(t *testing.T) {
			if got := isLineOrCol(tc); got {
				t.Fatalf("isLineOrCol(%q) = %v, want false", tc, got)
			}
		})
	}
}
