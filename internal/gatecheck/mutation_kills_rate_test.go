// Vision: Heavier mutation kill-rate scenarios: cross-file weighting, partial kills, and report normalization edges.
package gatecheck

import (
	"testing"
)

func TestParseMutationKillsReport_FlatMutations_WithPackageAndFile(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED", "file": "pkg/a.go", "package": "pkg"},
			{"status": "KILLED", "file": "pkg/a.go", "package": "pkg"},
			{"status": "LIVED", "file": "pkg/b.go", "package": "pkg"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)

	if check.TotalKilled != 2 {
		t.Fatalf("expected 2 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.TotalLived)
	}
	if len(check.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(check.Files))
	}
	if len(check.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(check.Packages))
	}

	denominator := check.TotalKilled + check.TotalLived
	expectedRate := float64(check.TotalKilled) / float64(denominator) * 100
	if check.KillRatePercent != expectedRate {
		t.Errorf("expected kill rate %.2f%%, got %.2f%%", expectedRate, check.KillRatePercent)
	}
}

func TestParseMutationKillsReport_FlatMutations_MultiplePackages(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED", "file": "pkg1/a.go", "package": "pkg1"},
			{"status": "LIVED", "file": "pkg2/b.go", "package": "pkg2"},
			{"status": "TIMED_OUT", "file": "pkg3/c.go", "package": "pkg3"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)

	if len(check.Packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(check.Packages))
	}

	pkgMap := make(map[string]*PackageMutationStats)
	for i := range check.Packages {
		pkgMap[check.Packages[i].Package] = &check.Packages[i]
	}

	pkg1, ok := pkgMap["pkg1"]
	if !ok {
		t.Fatalf("pkgMap missing key pkg1; keys present: %v", pkgMap)
	}
	if pkg1.Killed != 1 {
		t.Errorf("expected pkg1 to have 1 killed, got %d", pkg1.Killed)
	}

	pkg2, ok := pkgMap["pkg2"]
	if !ok {
		t.Fatalf("pkgMap missing key pkg2; keys present: %v", pkgMap)
	}
	if pkg2.Lived != 1 {
		t.Errorf("expected pkg2 to have 1 lived, got %d", pkg2.Lived)
	}

	pkg3, ok := pkgMap["pkg3"]
	if !ok {
		t.Fatalf("pkgMap missing key pkg3; keys present: %v", pkgMap)
	}
	if pkg3.TimedOut != 1 {
		t.Errorf("expected pkg3 to have 1 timed_out, got %d", pkg3.TimedOut)
	}
}

func TestParseMutationKillsReport_FlatMutations_AllStatuses(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED", "file": "a.go", "package": "pkg"},
			{"status": "LIVED", "file": "a.go", "package": "pkg"},
			{"status": "NOT_COVERED", "file": "a.go", "package": "pkg"},
			{"status": "TIMED_OUT", "file": "a.go", "package": "pkg"},
			{"status": "NOT_VIABLE", "file": "a.go", "package": "pkg"},
			{"status": "RUNNABLE", "file": "a.go", "package": "pkg"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)

	if check.TotalKilled != 1 {
		t.Errorf("expected 1 killed, got %d", check.TotalKilled)
	}
	if check.TotalLived != 1 {
		t.Errorf("expected 1 lived, got %d", check.TotalLived)
	}
	if check.TotalNotCovered != 1 {
		t.Errorf("expected 1 not_covered, got %d", check.TotalNotCovered)
	}
	if check.TotalTimedOut != 1 {
		t.Errorf("expected 1 timed_out, got %d", check.TotalTimedOut)
	}
	if check.TotalNotViable != 1 {
		t.Errorf("expected 1 not_viable, got %d", check.TotalNotViable)
	}
	if check.TotalRunnable != 1 {
		t.Errorf("expected 1 runnable, got %d", check.TotalRunnable)
	}
}

func TestParseMutationKillsReport_FilenameFallback(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"filename": "pkg/baz.go",
				"mutations": [{"status": "KILLED"}]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(check.Files))
	}
	if check.Files[0].File != "pkg/baz.go" {
		t.Fatalf("expected pkg/baz.go from fallback, got %s", check.Files[0].File)
	}
}

func TestParseMutationKillsReport_KillRateCalculation_Perfect(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "KILLED"},
					{"status": "KILLED"},
					{"status": "KILLED"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.KillRatePercent != 100.0 {
		t.Fatalf("expected 100%%, got %.2f%%", check.KillRatePercent)
	}
}

func TestParseMutationKillsReport_KillRateCalculation_Zero(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "LIVED"},
					{"status": "LIVED"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.KillRatePercent != 0.0 {
		t.Fatalf("expected 0%%, got %.2f%%", check.KillRatePercent)
	}
}

func TestParseMutationKillsReport_KillRateCalculation_IgnoresNonKilledOrLived(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "test.go",
				"mutations": [
					{"status": "KILLED"},
					{"status": "LIVED"},
					{"status": "NOT_COVERED"},
					{"status": "TIMED_OUT"},
					{"status": "NOT_VIABLE"},
					{"status": "RUNNABLE"}
				]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if check.KillRatePercent != 50.0 {
		t.Fatalf("expected 50%% (1 killed / 2 total killed+lived), got %.2f%%", check.KillRatePercent)
	}
}

func TestParseMutationKillsReport_MultipleFiles_SortedByPath(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{"file_name": "z.go", "mutations": [{"status": "KILLED"}]},
			{"file_name": "a.go", "mutations": [{"status": "KILLED"}]},
			{"file_name": "m.go", "mutations": [{"status": "KILLED"}]}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(check.Files))
	}
	if check.Files[0].File != "a.go" || check.Files[1].File != "m.go" || check.Files[2].File != "z.go" {
		fileNames := []string{check.Files[0].File, check.Files[1].File, check.Files[2].File}
		t.Fatalf("expected sorted files [a.go, m.go, z.go], got %v", fileNames)
	}
}

func TestParseMutationKillsReport_MultiplePackages_SortedByPath(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{"file_name": "z.go", "package": "z/pkg", "mutations": [{"status": "KILLED"}]},
			{"file_name": "a.go", "package": "a/pkg", "mutations": [{"status": "KILLED"}]},
			{"file_name": "m.go", "package": "m/pkg", "mutations": [{"status": "KILLED"}]}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(check.Packages))
	}
	pkgNames := []string{check.Packages[0].Package, check.Packages[1].Package, check.Packages[2].Package}
	p0, p1, p2 := check.Packages[0].Package, check.Packages[1].Package, check.Packages[2].Package
	if p0 != "a/pkg" || p1 != "m/pkg" || p2 != "z/pkg" {
		t.Fatalf("expected sorted packages, got %v", pkgNames)
	}
}

func TestParseMutationKillsReport_FilenamePrimaryPrecedence(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"files": [
			{
				"file_name": "primary.go",
				"filename": "fallback.go",
				"mutations": [{"status": "KILLED"}]
			}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(check.Files))
	}
	if check.Files[0].File != "primary.go" {
		t.Fatalf("expected primary.go, got %s", check.Files[0].File)
	}
}

func TestParseMutationKillsReport_FlatMutationsUnknownFile(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"mutations": [
			{"status": "KILLED"},
			{"status": "LIVED"}
		]
	}`
	check := parseMutationKillsReportString(t, jsonData)
	if len(check.Files) != 1 {
		t.Fatalf("expected 1 file entry for unknown file, got %d", len(check.Files))
	}
	if check.Files[0].File != "unknown" {
		t.Fatalf("expected file named 'unknown', got %s", check.Files[0].File)
	}
	if check.Files[0].Killed != 1 {
		t.Fatalf("expected 1 killed, got %d", check.Files[0].Killed)
	}
	if check.Files[0].Lived != 1 {
		t.Fatalf("expected 1 lived, got %d", check.Files[0].Lived)
	}
}

// Tests for previously surviving mutations in mutation_kills_helpers.go

func TestMaxLivedAllowedForMinKillRate_ZeroKilled(t *testing.T) {
	t.Parallel()
	// Line 157: killed < 0 returns 0, but killed == 0 with positive minRate
	result := maxLivedAllowedForMinKillRate(0, 90)
	if result != 0 {
		t.Fatalf("maxLivedAllowedForMinKillRate(0, 90) = %d, want 0", result)
	}
}

func TestMaxLivedAllowedForMinKillRate_MinRateZero(t *testing.T) {
	t.Parallel()
	// Line 157: minRate <= 0 returns 0
	result := maxLivedAllowedForMinKillRate(10, 0)
	if result != 0 {
		t.Fatalf("maxLivedAllowedForMinKillRate(10, 0) = %d, want 0", result)
	}
}

func TestMaxLivedAllowedForMinKillRate_MinRateHundred(t *testing.T) {
	t.Parallel()
	// Line 160-161: minRate >= 100 returns 0
	result := maxLivedAllowedForMinKillRate(10, 100)
	if result != 0 {
		t.Fatalf("maxLivedAllowedForMinKillRate(10, 100) = %d, want 0", result)
	}
	result = maxLivedAllowedForMinKillRate(10, 101)
	if result != 0 {
		t.Fatalf("maxLivedAllowedForMinKillRate(10, 101) = %d, want 0", result)
	}
}

func TestMaxLivedAllowedForMinKillRate_MathBoundary(t *testing.T) {
	t.Parallel()
	// Line 163: integer division boundary
	// killed=9, minRate=90 -> 9 * 10 / 90 = 1
	result := maxLivedAllowedForMinKillRate(9, 90)
	if result != 1 {
		t.Fatalf("maxLivedAllowedForMinKillRate(9, 90) = %d, want 1", result)
	}
	// killed=10, minRate=90 -> 10 * 10 / 90 = 1 (truncated)
	result = maxLivedAllowedForMinKillRate(10, 90)
	if result != 1 {
		t.Fatalf("maxLivedAllowedForMinKillRate(10, 90) = %d, want 1", result)
	}
}

func TestAppendTimedOutMutationDetail_CapAtMax(t *testing.T) {
	t.Parallel()
	// Lines 166-171: Cap at maxTimedOutDetailsPerFile (12)
	fs := &FileMutationStats{File: "test.go"}
	for i := 0; i < 15; i++ {
		mut := map[string]any{
			"type":   "TEST_TYPE",
			"line":   i,
			"column": 1,
		}
		appendTimedOutMutationDetail(fs, mut)
	}
	if len(fs.TimedOutDetails) != maxTimedOutDetailsPerFile {
		t.Fatalf("expected %d details, got %d", maxTimedOutDetailsPerFile, len(fs.TimedOutDetails))
	}
}

func TestAppendTimedOutMutationDetail_NilFileStats(t *testing.T) {
	t.Parallel()
	// Line 167: nil check should return early without panic
	mut := map[string]any{"type": "TEST"}
	appendTimedOutMutationDetail(nil, mut) // Should not panic
}

func TestFormatTimedOutMutationDetail_NoLocation(t *testing.T) {
	t.Parallel()
	// Lines 173-184: format without line/col returns just type
	mut := map[string]any{"type": "ARITHMETIC_BASE"}
	detail := formatTimedOutMutationDetail(mut)
	if detail != "ARITHMETIC_BASE" {
		t.Fatalf("expected 'ARITHMETIC_BASE', got %q", detail)
	}
}

func TestFormatTimedOutMutationDetail_WhitespaceType(t *testing.T) {
	t.Parallel()
	// Line 175: TrimSpace on type
	mut := map[string]any{
		"type":   "  CONDITIONALS_NEGATION  ",
		"line":   10,
		"column": 5,
	}
	detail := formatTimedOutMutationDetail(mut)
	want := "CONDITIONALS_NEGATION line 10 col 5"
	if detail != want {
		t.Fatalf("expected %q, got %q", want, detail)
	}
}

func TestFormatTimedOutMutationDetail_EmptyType(t *testing.T) {
	t.Parallel()
	// Lines 176-178: Empty type defaults to UNKNOWN_TYPE
	mut := map[string]any{
		"type":   "",
		"line":   10,
		"column": 5,
	}
	detail := formatTimedOutMutationDetail(mut)
	want := "UNKNOWN_TYPE line 10 col 5"
	if detail != want {
		t.Fatalf("expected %q, got %q", want, detail)
	}
}

func TestJsonNumberToInt_Int64(t *testing.T) {
	t.Parallel()
	// Lines 187-197: int64 conversion
	result := jsonNumberToInt(int64(42))
	if result != 42 {
		t.Fatalf("jsonNumberToInt(int64(42)) = %d, want 42", result)
	}
}

func TestJsonNumberToInt_Default(t *testing.T) {
	t.Parallel()
	// Lines 195-197: default case returns 0
	result := jsonNumberToInt("string")
	if result != 0 {
		t.Fatalf("jsonNumberToInt(\"string\") = %d, want 0", result)
	}
	result = jsonNumberToInt(nil)
	if result != 0 {
		t.Fatalf("jsonNumberToInt(nil) = %d, want 0", result)
	}
}

func TestNormalizePackageName_NonEmpty(t *testing.T) {
	t.Parallel()
	// Line 12: non-empty returns unchanged
	result := normalizePackageName("github.com/test/pkg")
	if result != "github.com/test/pkg" {
		t.Fatalf("normalizePackageName = %q, want 'github.com/test/pkg'", result)
	}
}

func TestNormalizeStatus_WhitespaceTrim(t *testing.T) {
	t.Parallel()
	// Line 22: TrimSpace
	result, err := normalizeStatus("  KILLED  ")
	if err != nil {
		t.Fatalf("normalizeStatus: %v", err)
	}
	if result != "KILLED" {
		t.Fatalf("normalizeStatus = %q, want 'KILLED'", result)
	}
}

func TestComputeKillRate_ZeroDenominator(t *testing.T) {
	t.Parallel()
	// Line 122: denominator == 0 should not modify KillRatePercent
	check := &MutationKillsCheck{
		KillRatePercent: -1, // sentinel to verify no modification
	}
	computeKillRate(check)
	if check.KillRatePercent != -1 {
		t.Fatalf("KillRatePercent modified with zero denominator, got %.2f", check.KillRatePercent)
	}
}
