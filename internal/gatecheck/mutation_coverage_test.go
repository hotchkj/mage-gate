// Vision: Mutation coverage threshold uses the same parsed kill-stats model as kill rate.
package gatecheck

import (
	"errors"
	"fmt"
	"testing"
)

func TestCheckMutationCoverage_Disabled(t *testing.T) {
	t.Parallel()
	ch := &MutationKillsCheck{TotalNotCovered: 10, TotalRunnable: 0}
	if err := CheckMutationCoverage(ch, 0); err != nil {
		t.Fatalf("Min 0 must disable: %v", err)
	}
}

func TestCheckMutationCoverage_NoMutations(t *testing.T) {
	t.Parallel()
	err := CheckMutationCoverage(&MutationKillsCheck{}, 1)
	if err == nil {
		t.Fatal("expected error when report is empty")
	}
	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("want ErrMutationCoverageFailed, got %v", err)
	}
}

func TestCheckMutationCoverage_PassAndFail(t *testing.T) {
	t.Parallel()
	// 3 mutants: 2 covered (RUNNABLE), 1 NOT_COVERED → 66.7%
	ch := &MutationKillsCheck{
		TotalRunnable:   2,
		TotalNotCovered: 1,
	}
	if err := CheckMutationCoverage(ch, 66); err != nil {
		t.Fatalf("expected pass at 66%%: %v", err)
	}
	failErr := CheckMutationCoverage(ch, 67)
	if failErr == nil {
		t.Fatal("expected fail at 67%%")
	}
	if !errors.Is(failErr, ErrMutationCoverageFailed) {
		t.Fatalf("want ErrMutationCoverageFailed, got %v", failErr)
	}
}

func TestCheckMutationCoverageWithScope_DelegatesWhenNoScope(t *testing.T) {
	t.Parallel()
	ch := &MutationKillsCheck{
		TotalRunnable:   2,
		TotalNotCovered: 1,
		Files: []FileMutationStats{
			{File: "pkg/a.go", Runnable: 2, NotCovered: 1},
		},
	}
	if err := CheckMutationCoverageWithScope(ch, 66, nil, nil); err != nil {
		t.Fatalf("66%%: %v", err)
	}
	if err := CheckMutationCoverageWithScope(ch, 66, []string{}, nil); err != nil {
		t.Fatalf("empty exclude: %v", err)
	}
}

func TestCheckMutationCoverageWithScope_ExcludeChangesOutcome(t *testing.T) {
	t.Parallel()
	// vendor: 5 NOT_COVERED; pkg: 1 RUNNABLE. Global 1/6 ≈ 16.7%%; without vendor, 1/1 = 100%%.
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	if err := CheckMutationCoverageWithScope(check, 50, nil, nil); err == nil {
		t.Fatal("unscoped: expected failure at 50%%")
	}
	if err := CheckMutationCoverageWithScope(check, 50, []string{"vendor"}, nil); err != nil {
		t.Fatalf("exclude vendor: %v", err)
	}
}

func TestCheckMutationCoverageWithScope_TestFilePatternsChangeOutcome(t *testing.T) {
	t.Parallel()
	// a_test.go: 6 NOT_COVERED; a.go: 1 RUNNABLE. Global 1/7; skip *_test.go → 1/1.
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	if err := CheckMutationCoverageWithScope(check, 50, nil, nil); err == nil {
		t.Fatal("unscoped: expected failure at 50%% (coverage ~14%%)")
	}
	if err := CheckMutationCoverageWithScope(check, 50, nil, []string{"*_test.go"}); err != nil {
		t.Fatalf("skip test sources: %v", err)
	}
}

func TestCheckKillsReportSiteBudget_ScopedInclusionMatchesScopedCoverage_Exclude(t *testing.T) {
	t.Parallel()
	// With vendor excluded, only pkg/x.go counts: 2 RUNNABLE → 100% coverage at 50% passes;
	// per-file site cap 1 must still fail because pkg has 2 sites.
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"},{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	exclude := []string{"vendor"}
	if err := CheckMutationCoverageWithScope(check, 50, exclude, nil); err != nil {
		t.Fatalf("scoped coverage: %v", err)
	}
	if err := CheckKillsReportSiteBudget(check, 2, exclude, nil); err != nil {
		t.Fatalf("expected pass at maxSites 2: %v", err)
	}
	siteErr := CheckKillsReportSiteBudget(check, 1, exclude, nil)
	if siteErr == nil {
		t.Fatal("expected site budget failure at maxSites 1 (pkg has 2 sites)")
	}
	if !errors.Is(siteErr, ErrMutationSitesFailed) {
		t.Fatalf("want ErrMutationSitesFailed, got %v", siteErr)
	}
}

func TestCheckKillsReportSiteBudget_ScopedInclusionMatchesScopedCoverage_TestFilePatterns(t *testing.T) {
	t.Parallel()
	// With *_test.go skipped, only pkg/a.go counts: 1 site. Unscoped, a_test.go has 6 sites and
	// fails maxSites 5; scoped pass uses the same file set as [CheckMutationCoverageWithScope].
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	patterns := []string{"*_test.go"}
	if err := CheckMutationCoverageWithScope(check, 50, nil, patterns); err != nil {
		t.Fatalf("scoped coverage: %v", err)
	}
	if err := CheckKillsReportSiteBudget(check, 1, nil, patterns); err != nil {
		t.Fatalf("expected pass: only a.go with 1 site: %v", err)
	}
	unscopedErr := CheckKillsReportSiteBudget(check, 5, nil, nil)
	if unscopedErr == nil {
		t.Fatal("unscoped: expected failure (a_test.go has 6 sites > 5)")
	}
	if !errors.Is(unscopedErr, ErrMutationSitesFailed) {
		t.Fatalf("want ErrMutationSitesFailed, got %v", unscopedErr)
	}
}

func TestMutationCoverageScopedTotals_SumOfSiteCountsMatchesTotal(t *testing.T) {
	t.Parallel()
	// Invariant: totals from [mutationCoverageCountsForFileScope] match the sum of site counts
	// over the same inclusion set (single iterator in [forEachScopedNonEmptyMutationFile]).
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[{"status":"RUNNABLE"}]},
		{"file_name":"pkg/x.go","package":"p","mutations":[
			{"status":"RUNNABLE"},{"status":"NOT_COVERED"}
		]}
	]}`
	check := parseMutationKillsReportString(t, json)
	exclude := []string{"vendor"}
	total, _ := mutationCoverageCountsForFileScope(check, exclude, nil)
	var sumN int
	forEachScopedNonEmptyMutationFile(check, exclude, nil, func(_ *FileMutationStats, n int) {
		sumN += n
	})
	if sumN != total {
		t.Fatalf("sum of scoped site counts %d != coverage total %d", sumN, total)
	}
}

func TestFormatMutationCoverageReport_EmptyCheck(t *testing.T) {
	t.Parallel()
	if result := FormatMutationCoverageReport(nil); result != "" {
		t.Fatalf("expected empty for nil check, got %q", result)
	}
	if result := FormatMutationCoverageReport(&MutationKillsCheck{}); result != "" {
		t.Fatalf("expected empty for empty check, got %q", result)
	}
}

func TestFormatMutationCoverageReport_NoFiles(t *testing.T) {
	t.Parallel()
	check := &MutationKillsCheck{Files: []FileMutationStats{}}
	if result := FormatMutationCoverageReport(check); result != "" {
		t.Fatalf("expected empty for no files, got %q", result)
	}
}

func TestFormatMutationCoverageReport_SingleFile(t *testing.T) {
	t.Parallel()
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: "pkg/a.go", Killed: 2, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0},
		},
	}
	result := FormatMutationCoverageReport(check)
	want := "Worst coverage files:\n  66.7%  pkg/a.go (1/3 not covered)\n"
	if result != want {
		t.Fatalf("result = %q, want %q", result, want)
	}
}

func TestFormatMutationCoverageReport_MultipleFilesSorted(t *testing.T) {
	t.Parallel()
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: "pkg/b.go", Killed: 4, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0}, // 80%
			{File: "pkg/a.go", Killed: 1, Lived: 0, NotCovered: 2, TimedOut: 0, NotViable: 0, Runnable: 0}, // 33.3%
			{File: "pkg/c.go", Killed: 3, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0}, // 75%
		},
	}
	result := FormatMutationCoverageReport(check)
	want := "Worst coverage files:\n" +
		"  33.3%  pkg/a.go (2/3 not covered)\n" +
		"  75.0%  pkg/c.go (1/4 not covered)\n" +
		"  80.0%  pkg/b.go (1/5 not covered)\n"
	if result != want {
		t.Fatalf("result = %q, want %q", result, want)
	}
}

func TestFormatMutationCoverageReport_Limit(t *testing.T) {
	t.Parallel()
	files := make([]FileMutationStats, 15)
	for i := range files {
		files[i] = FileMutationStats{
			File:       fmt.Sprintf("pkg/file%d.go", i),
			Killed:     0,
			Lived:      0,
			NotCovered: 1,
			TimedOut:   0,
			NotViable:  0,
			Runnable:   0,
		}
	}
	check := &MutationKillsCheck{Files: files}
	result := FormatMutationCoverageReport(check)
	const want = "Worst coverage files:\n" +
		"  0.0%  pkg/file0.go (1/1 not covered)\n" +
		"  0.0%  pkg/file1.go (1/1 not covered)\n" +
		"  0.0%  pkg/file10.go (1/1 not covered)\n" +
		"  0.0%  pkg/file11.go (1/1 not covered)\n" +
		"  0.0%  pkg/file12.go (1/1 not covered)\n" +
		"  0.0%  pkg/file13.go (1/1 not covered)\n" +
		"  0.0%  pkg/file14.go (1/1 not covered)\n" +
		"  0.0%  pkg/file2.go (1/1 not covered)\n" +
		"  0.0%  pkg/file3.go (1/1 not covered)\n" +
		"  0.0%  pkg/file4.go (1/1 not covered)\n" +
		"  ... and 5 more files\n"
	if result != want {
		t.Fatalf("result = %q, want %q", result, want)
	}
}

const testPkgA = "pkg/a.go"

func TestCalculateFileCoverage(t *testing.T) {
	t.Parallel()
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: testPkgA, Killed: 2, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0},
			{
				File: "pkg/b.go", Killed: 2, Lived: 0, NotCovered: 0,
				TimedOut: 0, NotViable: 0, Runnable: 0, // skipped (fully covered)
			},
			{
				File: "pkg/c.go", Killed: 0, Lived: 0, NotCovered: 0,
				TimedOut: 0, NotViable: 0, Runnable: 0, // skipped (empty)
			},
		},
	}
	files := calculateFileCoverage(check)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].file != testPkgA {
		t.Fatalf("expected %s, got %s", testPkgA, files[0].file)
	}
	if files[0].notCovered != 1 {
		t.Fatalf("expected notCovered=1, got %d", files[0].notCovered)
	}
}

func TestSortFilesByCoverage(t *testing.T) {
	t.Parallel()
	files := []fileCoverage{
		{file: "pkg/b.go", percent: 80.0},
		{file: testPkgA, percent: 33.3},
		{file: "pkg/c.go", percent: 80.0},
	}
	sortFilesByCoverage(files)
	if files[0].file != testPkgA {
		t.Fatalf("expected first file to be %s (lowest percent), got %s", testPkgA, files[0].file)
	}
	if files[0].percent != 33.3 {
		t.Fatalf("expected first percent to be 33.3, got %f", files[0].percent)
	}
}

func TestMinInt(t *testing.T) {
	t.Parallel()
	if min(5, 10) != 5 {
		t.Fatal("expected min(5, 10) = 5")
	}
	if min(10, 5) != 5 {
		t.Fatal("expected min(10, 5) = 5")
	}
	if min(5, 5) != 5 {
		t.Fatal("expected min(5, 5) = 5")
	}
}

// Tests for previously surviving mutations in mutation_coverage_scope.go

func TestForEachScopedNonEmptyMutationFile_SkipsZeroSiteCount(t *testing.T) {
	t.Parallel()
	// Line 56-58: skips files with zero site count
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: "pkg/zero.go", Killed: 0, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
			{File: "pkg/nonzero.go", Killed: 1, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
		},
	}
	var visited []string
	forEachScopedNonEmptyMutationFile(check, nil, nil, func(row *FileMutationStats, n int) {
		visited = append(visited, row.File)
	})
	if len(visited) != 1 || visited[0] != "pkg/nonzero.go" {
		t.Fatalf("expected only nonzero.go, got %v", visited)
	}
}

func TestShouldSkipMutationFile_NoTestFilePatterns(t *testing.T) {
	t.Parallel()
	// Line 82: len(testFilePatterns) == 0 skips pattern check
	// With no patterns, file should not be skipped based on patterns
	result := shouldSkipMutationFile("pkg/file.go", nil, nil)
	if result {
		t.Fatal("should not skip file when no patterns provided")
	}
	// With exclude only - path must contain segment with proper separators
	result = shouldSkipMutationFile("pkg/vendor/file.go", []string{"vendor"}, nil)
	if !result {
		t.Fatal("should skip vendor file")
	}
}

func TestShouldSkipMutationFile_WithTestFilePatterns(t *testing.T) {
	t.Parallel()
	// Line 82: with patterns, check matches
	result := shouldSkipMutationFile("pkg/file_test.go", nil, []string{"*_test.go"})
	if !result {
		t.Fatal("should skip test file when pattern matches")
	}
	result = shouldSkipMutationFile("pkg/file.go", nil, []string{"*_test.go"})
	if result {
		t.Fatal("should not skip non-test file")
	}
}

func TestFormatMutationCoverageReport_NilCheck(t *testing.T) {
	t.Parallel()
	// Line 92: nil check returns empty
	result := FormatMutationCoverageReport(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil check, got %q", result)
	}
}

func TestFormatMutationCoverageReport_EmptyFiles(t *testing.T) {
	t.Parallel()
	// Line 92-93: empty files returns empty
	check := &MutationKillsCheck{Files: []FileMutationStats{}}
	result := FormatMutationCoverageReport(check)
	if result != "" {
		t.Fatalf("expected empty string for empty files, got %q", result)
	}
}

func TestFormatMutationCoverageReport_NoValidFiles(t *testing.T) {
	t.Parallel()
	// Lines 97-98, 123-133: all files filtered returns empty
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: "pkg/unknown", Killed: 1},                   // unknown file skipped
			{File: "pkg/covered.go", Killed: 1, NotCovered: 0}, // fully covered skipped
			{File: "pkg/empty.go", Killed: 0, NotCovered: 0},   // zero total skipped
		},
	}
	result := FormatMutationCoverageReport(check)
	if result != "" {
		t.Fatalf("expected empty string when no valid files, got %q", result)
	}
}

func TestCalculateFileCoverage_ExceedsLimit(t *testing.T) {
	t.Parallel()
	// Lines 106-112: limit truncation
	check := &MutationKillsCheck{
		Files: []FileMutationStats{
			{File: "pkg/a.go", Killed: 0, NotCovered: 1},
			{File: "pkg/b.go", Killed: 0, NotCovered: 1},
			{File: "pkg/c.go", Killed: 0, NotCovered: 1},
			{File: "pkg/d.go", Killed: 0, NotCovered: 1},
			{File: "pkg/e.go", Killed: 0, NotCovered: 1},
			{File: "pkg/f.go", Killed: 0, NotCovered: 1},
			{File: "pkg/g.go", Killed: 0, NotCovered: 1},
			{File: "pkg/h.go", Killed: 0, NotCovered: 1},
			{File: "pkg/i.go", Killed: 0, NotCovered: 1},
			{File: "pkg/j.go", Killed: 0, NotCovered: 1},
			{File: "pkg/k.go", Killed: 0, NotCovered: 1},
			{File: "pkg/l.go", Killed: 0, NotCovered: 1},
		},
	}
	result := FormatMutationCoverageReport(check)
	const want = "Worst coverage files:\n" +
		"  0.0%  pkg/a.go (1/1 not covered)\n" +
		"  0.0%  pkg/b.go (1/1 not covered)\n" +
		"  0.0%  pkg/c.go (1/1 not covered)\n" +
		"  0.0%  pkg/d.go (1/1 not covered)\n" +
		"  0.0%  pkg/e.go (1/1 not covered)\n" +
		"  0.0%  pkg/f.go (1/1 not covered)\n" +
		"  0.0%  pkg/g.go (1/1 not covered)\n" +
		"  0.0%  pkg/h.go (1/1 not covered)\n" +
		"  0.0%  pkg/i.go (1/1 not covered)\n" +
		"  0.0%  pkg/j.go (1/1 not covered)\n" +
		"  ... and 2 more files\n"
	if result != want {
		t.Fatalf("result = %q, want %q", result, want)
	}
}

func TestSortFilesByCoverage_TieBreakByFile(t *testing.T) {
	t.Parallel()
	// Lines 153-158: tie-break by file name
	files := []fileCoverage{
		{file: "pkg/z.go", percent: 50.0},
		{file: "pkg/a.go", percent: 50.0},
		{file: "pkg/m.go", percent: 50.0},
	}
	sortFilesByCoverage(files)
	if files[0].file != "pkg/a.go" || files[1].file != "pkg/m.go" || files[2].file != "pkg/z.go" {
		t.Fatalf("expected sorted by file name on tie, got %v", files)
	}
}

func TestFormatMutationCoverageResultRows_NilResult(t *testing.T) {
	t.Parallel()
	// Lines 165-167: nil result returns empty
	result := FormatMutationCoverageResultRows(nil, 10)
	if result != "" {
		t.Fatalf("expected empty string for nil result, got %q", result)
	}
}

func TestFormatMutationCoverageResultRows_EmptyRows(t *testing.T) {
	t.Parallel()
	// Lines 165-167: empty rows returns empty
	emptyResult := &MutationCoverageResult{WorstFileRows: []MutationCoverageRow{}}
	result := FormatMutationCoverageResultRows(emptyResult, 10)
	if result != "" {
		t.Fatalf("expected empty string for empty rows, got %q", result)
	}
}

func TestFormatMutationCoverageResultRows_ZeroLimit(t *testing.T) {
	t.Parallel()
	// Lines 169-171: limit <= 0 defaults to MaxWorstFileRows
	result := &MutationCoverageResult{
		WorstFileRows: []MutationCoverageRow{
			{File: testPkgA, Percent: 50.0, Total: 2, NotCovered: 1},
		},
	}
	output := FormatMutationCoverageResultRows(result, 0)
	want := "Worst coverage files:\n  50.0%  " + testPkgA + " (1/2 not covered)\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}

func TestFormatMutationCoverageResultRows_Truncation(t *testing.T) {
	t.Parallel()
	// Lines 187-189: truncation message
	rows := make([]MutationCoverageRow, 0, 15)
	for i := 0; i < 15; i++ {
		row := MutationCoverageRow{
			File: fmt.Sprintf("pkg/%d.go", i), Percent: float64(i), Total: 2, NotCovered: 1,
		}
		rows = append(rows, row)
	}
	result := &MutationCoverageResult{WorstFileRows: rows}
	output := FormatMutationCoverageResultRows(result, 10)
	const want = "Worst coverage files:\n" +
		"  0.0%  pkg/0.go (1/2 not covered)\n" +
		"  1.0%  pkg/1.go (1/2 not covered)\n" +
		"  2.0%  pkg/2.go (1/2 not covered)\n" +
		"  3.0%  pkg/3.go (1/2 not covered)\n" +
		"  4.0%  pkg/4.go (1/2 not covered)\n" +
		"  5.0%  pkg/5.go (1/2 not covered)\n" +
		"  6.0%  pkg/6.go (1/2 not covered)\n" +
		"  7.0%  pkg/7.go (1/2 not covered)\n" +
		"  8.0%  pkg/8.go (1/2 not covered)\n" +
		"  9.0%  pkg/9.go (1/2 not covered)\n" +
		"  ... and 5 more files\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}
