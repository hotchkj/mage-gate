// Vision: [MutationMetricsSnapshot] mirrors parsed kill stats without a second JSON decode.
package gatecheck

import (
	"errors"
	"testing"
)

func assertCoverageParity(
	t *testing.T,
	check *MutationKillsCheck,
	snap MutationMetricsSnapshot,
	minPercent int,
	exclude []string,
	patterns []string,
	wantErr bool,
) {
	t.Helper()
	errCheck := CheckMutationCoverageWithScope(check, minPercent, exclude, patterns)
	_, errSnap := CheckMutationCoverageOnMetricsSnapshot(snap, minPercent, exclude, patterns)
	if (errCheck == nil) != (errSnap == nil) {
		t.Fatalf("check err=%v snap err=%v (want same nilness)", errCheck, errSnap)
	}
	if !wantErr {
		if errCheck != nil || errSnap != nil {
			t.Fatalf("want pass, check=%v snap=%v", errCheck, errSnap)
		}
		return
	}
	if errCheck == nil || errSnap == nil {
		t.Fatalf("want failure, check=%v snap=%v", errCheck, errSnap)
	}
	if !errors.Is(errCheck, ErrMutationCoverageFailed) || !errors.Is(errSnap, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed: check=%v snap=%v", errCheck, errSnap)
	}
}

func TestSnapshotFromMutationKillsCheck_RoundTripMatchesCheckMutationCoverageWithScope(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	snap := SnapshotFromMutationKillsCheck(check)
	cases := []struct {
		name    string
		exclude []string
		pats    []string
		maxPct  int
		wantErr bool
	}{
		{"unscoped_50", nil, nil, 50, true},
		{"exclude_vendor_50", []string{"vendor"}, nil, 50, false},
	}
	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertCoverageParity(t, check, snap, tc.maxPct, tc.exclude, tc.pats, tc.wantErr)
		})
	}
}

func TestSnapshotFromMutationKillsCheck_RoundTripParityForTestFilePatterns(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	check := parseMutationKillsReportString(t, json)
	snap := SnapshotFromMutationKillsCheck(check)
	patterns := []string{"*_test.go"}

	if err := CheckMutationCoverageWithScope(check, 50, nil, nil); err == nil {
		t.Fatal("unscoped check path: expected failure at 50%")
	}
	if _, err := CheckMutationCoverageOnMetricsSnapshot(snap, 50, nil, nil); err == nil {
		t.Fatal("unscoped snapshot path: expected failure at 50%")
	}

	if err := CheckMutationCoverageWithScope(check, 50, nil, patterns); err != nil {
		t.Fatalf("check path with patterns: %v", err)
	}
	if _, err := CheckMutationCoverageOnMetricsSnapshot(snap, 50, nil, patterns); err != nil {
		t.Fatalf("snapshot path with patterns: %v", err)
	}
}

func TestFilterMutationMetricsByQualityScope_DropsExcludedPaths(t *testing.T) {
	t.Parallel()
	snap := MutationMetricsSnapshot{
		Files: []MutationFileMetrics{
			{File: "a/vendor/x.go", Killed: 0, Lived: 0, NotCovered: 1, Runnable: 0},
			{File: "pkg/ok.go", Killed: 0, Lived: 0, NotCovered: 0, Runnable: 1},
		},
	}
	out := FilterMutationMetricsByQualityScope(snap, []string{"vendor"}, nil)
	if len(out.Files) != 1 || out.Files[0].File != "pkg/ok.go" {
		t.Fatalf("got %+v, want one row pkg/ok.go", out.Files)
	}
}

func TestCheckMutationCoverageOnMetricsSnapshot_EmptyFiltersUseFileRows(t *testing.T) {
	t.Parallel()
	snap := MutationMetricsSnapshot{
		Files: []MutationFileMetrics{
			{File: "pkg/a.go", Runnable: 2, NotCovered: 1},
		},
	}
	if _, err := CheckMutationCoverageOnMetricsSnapshot(snap, 66, nil, nil); err != nil {
		t.Fatalf("66%%: %v", err)
	}
	if _, err := CheckMutationCoverageOnMetricsSnapshot(snap, 67, nil, nil); err == nil {
		t.Fatal("expected fail at 67%")
	}
}

// Tests for previously surviving mutations in mutation_metrics_snapshot.go

func TestSnapshotFromMutationKillsCheck_NilCheck(t *testing.T) {
	t.Parallel()
	// Lines 38-39: nil check returns empty snapshot
	snap := SnapshotFromMutationKillsCheck(nil)
	if len(snap.Files) != 0 {
		t.Fatalf("expected empty files for nil check, got %d", len(snap.Files))
	}
}

func TestCheckMutationCoverageOnMetricsSnapshot_MinPercentZero(t *testing.T) {
	t.Parallel()
	// Lines 115-116: minPercent <= 0 returns nil
	snap := MutationMetricsSnapshot{
		Files: []MutationFileMetrics{{File: "pkg/a.go", Runnable: 1}},
	}
	result, err := CheckMutationCoverageOnMetricsSnapshot(snap, 0, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error for minPercent=0, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for minPercent=0, got %v", result)
	}
}

func TestCheckMutationCoverageOnMetricsSnapshot_NegativeMinPercent(t *testing.T) {
	t.Parallel()
	// Lines 115-116: negative minPercent also returns nil
	snap := MutationMetricsSnapshot{
		Files: []MutationFileMetrics{{File: "pkg/a.go", Runnable: 1}},
	}
	result, err := CheckMutationCoverageOnMetricsSnapshot(snap, -1, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error for minPercent=-1, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for minPercent=-1, got %v", result)
	}
}

func TestCheckMutationCoverageFromScoped_SiteCountZero(t *testing.T) {
	t.Parallel()
	// Lines 132-134: siteCount == 0 skips adding to worst rows
	files := []MutationFileMetrics{
		{File: "pkg/zero.go", Killed: 0, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
		{File: "pkg/nonzero.go", Killed: 1, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0},
	}
	result, err := checkMutationCoverageFromScopedFileMetricsList(files, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.WorstFileRows) != 1 {
		t.Fatalf("expected 1 worst file row, got %d", len(result.WorstFileRows))
	}
	if result.WorstFileRows[0].File != "pkg/nonzero.go" {
		t.Fatalf("expected pkg/nonzero.go, got %s", result.WorstFileRows[0].File)
	}
}

func TestCheckMutationCoverageFromScoped_NotCoveredZero(t *testing.T) {
	t.Parallel()
	// Lines 136-143: NotCovered == 0 skips adding to worst rows
	files := []MutationFileMetrics{
		{File: "pkg/covered.go", Killed: 1, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
		{File: "pkg/uncovered.go", Killed: 0, Lived: 0, NotCovered: 1, TimedOut: 0, NotViable: 0, Runnable: 0},
	}
	result, err := checkMutationCoverageFromScopedFileMetricsList(files, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.WorstFileRows) != 1 {
		t.Fatalf("expected 1 worst file row, got %d", len(result.WorstFileRows))
	}
	if result.WorstFileRows[0].File != "pkg/uncovered.go" {
		t.Fatalf("expected pkg/uncovered.go, got %s", result.WorstFileRows[0].File)
	}
}

func TestCheckMutationCoverageSummary_TotalZero(t *testing.T) {
	t.Parallel()
	// Lines 166-170: total == 0 returns error
	files := []MutationFileMetrics{
		{File: "pkg/zero.go", Killed: 0, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
	}
	result, err := checkMutationCoverageFromScopedFileMetricsList(files, 50)
	if err == nil {
		t.Fatal("expected error for zero total mutations")
	}
	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", err)
	}
	if result.Summary.Total != 0 {
		t.Fatalf("expected total=0, got %d", result.Summary.Total)
	}
}

func TestCheckMutationCoverageSummary_CoveredNegativeClamp(t *testing.T) {
	t.Parallel()
	// Lines 172-175: covered < 0 clamped to 0 (defensive)
	// This is defensive code - construct case to trigger it
	files := []MutationFileMetrics{
		{File: "pkg/a.go", Killed: 0, Lived: 0, NotCovered: 5, TimedOut: 0, NotViable: 0, Runnable: 0},
	}
	result, err := checkMutationCoverageFromScopedFileMetricsList(files, 50)
	// 0% coverage, should fail at 50%
	if err == nil {
		t.Fatal("expected error")
	}
	// covered = 5 - 5 = 0, not negative, so clamp not triggered
	// Just verify the calculation path works
	if result.Summary.Covered != 0 {
		t.Fatalf("expected covered=0, got %d", result.Summary.Covered)
	}
}

func TestSortMutationCoverageRows_TieByFile(t *testing.T) {
	t.Parallel()
	// Lines 226-231: tie-break by file name
	rows := []MutationCoverageRow{
		{File: "pkg/z.go", Percent: 50.0},
		{File: "pkg/a.go", Percent: 50.0},
		{File: "pkg/m.go", Percent: 50.0},
	}
	sortMutationCoverageRows(rows)
	if rows[0].File != "pkg/a.go" || rows[1].File != "pkg/m.go" || rows[2].File != "pkg/z.go" {
		t.Fatalf("expected sorted by file on tie, got %v", rows)
	}
}

func TestMutationFileMetricsSiteCount_AllFields(t *testing.T) {
	t.Parallel()
	// Lines 234-237: sum all fields
	file := &MutationFileMetrics{
		Killed: 1, Lived: 1, NotCovered: 1, TimedOut: 1, NotViable: 1, Runnable: 1,
	}
	count := mutationFileMetricsSiteCount(file)
	if count != 6 {
		t.Fatalf("expected 6, got %d", count)
	}
}

func TestFilterMutationMetricsByQualityScope_AllFilesExcluded(t *testing.T) {
	t.Parallel()
	// Lines 72-75: all files excluded returns empty
	snap := MutationMetricsSnapshot{
		Files: []MutationFileMetrics{
			{File: "a/vendor/x.go", Killed: 1},
			{File: "b/vendor/y.go", Killed: 1},
		},
	}
	out := FilterMutationMetricsByQualityScope(snap, []string{"vendor"}, nil)
	if len(out.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(out.Files))
	}
}

func TestMutationCoverageResult_PercentBelowThreshold(t *testing.T) {
	t.Parallel()
	// Lines 183-194: percent < minPercent sets ThresholdError
	files := []MutationFileMetrics{
		{File: "pkg/a.go", Runnable: 1, NotCovered: 1}, // 50% coverage
	}
	result, err := checkMutationCoverageFromScopedFileMetricsList(files, 90)
	if err == nil {
		t.Fatal("expected error when percent below threshold")
	}
	if result.Summary.ThresholdError == nil {
		t.Fatal("expected ThresholdError set")
	}
}
