// Vision: MutationCoverage reads stored gremlins JSON using the same kill-stats model as mutation kills.
package gate

import (
	"bytes"
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func TestMutationCoverage_DisabledThresholdNoRead(t *testing.T) {
	t.Parallel()
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, "./...")
	out := MutationScanOutput{
		store:        store,
		stepID:       "missing",
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	// No artifact: would fail if the check ran; min 0 must short-circuit.
	if err := MutationCoverage(out, MinMutationCoverage(0)); err != nil {
		t.Fatalf("MinMutationCoverage(0): %v", err)
	}
}

func TestMutationCoverage_RejectsEmptyStepID(t *testing.T) {
	t.Parallel()
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, "./...")
	out := MutationScanOutput{
		store:        store,
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	err := MutationCoverage(out, MinMutationCoverage(1))
	if err == nil {
		t.Fatal("expected error for empty stepID")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v", err)
	}
}

func TestMutationCoverage_ThresholdOnStoredReport(t *testing.T) {
	t.Parallel()
	store := NewArtifactStore()
	stepID := "mutationsites-1"
	// 3 mutants: 2 covered (RUNNABLE) + 1 NOT_COVERED → 66.7%.
	const json = `{"mutations":[
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"NOT_COVERED"}
	]}`
	if err := store.Write(stepID, "mutations.json", []byte(json), Provenance{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
	scope := mustNewQualityScope(t, "./...")
	out := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scope,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		outputMode:   OutputModeVerbose,
	}
	if err := MutationCoverage(out, MinMutationCoverage(50)); err != nil {
		t.Fatalf("50%%: %v", err)
	}
	highThreshErr := MutationCoverage(out, MinMutationCoverage(90))
	if highThreshErr == nil {
		t.Fatal("expected failure at 90%%")
	}
	if !errors.Is(highThreshErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", highThreshErr)
	}
}

func TestMutationCoverage_ExcludeScopeChangesThresholdOutcome(t *testing.T) {
	t.Parallel()
	// Same report as [TestMutationCoverageFromKills_ExcludeScopeChangesCoverage]: unscoped
	// product coverage is poor; [Exclude]("vendor") leaves only the good row.
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	const stepID = "mutationsites-cov-vendor"
	store := NewArtifactStore()
	if err := store.Write(stepID, "mutations.json", []byte(json), Provenance{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outAll := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scopeAll,
		outputMode:   OutputModeVerbose,
	}
	if covErr := MutationCoverage(outAll, MinMutationCoverage(50)); covErr == nil {
		t.Fatal("unscoped: expected failure at 50%")
	} else if !errors.Is(covErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", covErr)
	}
	scopeNoVendor, err := NewQualityScope("./...", Exclude("vendor"))
	if err != nil {
		t.Fatal(err)
	}
	outProd := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scopeNoVendor,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		outputMode:   OutputModeVerbose,
	}
	if err := MutationCoverage(outProd, MinMutationCoverage(50)); err != nil {
		t.Fatalf("exclude vendor: %v", err)
	}
}

func TestMutationCoverage_TestFilePatternsChangeThresholdOutcome(t *testing.T) {
	t.Parallel()
	// Same report as [TestMutationCoverageFromKills_TestFilePatternsChangeCoverage]: unscoped
	// total is dominated by *_test.go NOT_COVERED; [TestFilePatterns] drops that file.
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	const stepID = "mutationsites-cov-tf"
	store := NewArtifactStore()
	if err := store.Write(stepID, "mutations.json", []byte(json), Provenance{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outAll := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scopeAll,
		outputMode:   OutputModeVerbose,
	}
	if covErr := MutationCoverage(outAll, MinMutationCoverage(50)); covErr == nil {
		t.Fatal("unscoped: expected failure at 50%")
	} else if !errors.Is(covErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", covErr)
	}
	scopeTestSkip, err := NewQualityScope("./...", TestFilePatterns("*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	outSkip := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scopeTestSkip,
		pathFilters:  testMutationPathFilters(nil, []string{"*_test.go"}),
		outputMode:   OutputModeVerbose,
	}
	if err := MutationCoverage(outSkip, MinMutationCoverage(50)); err != nil {
		t.Fatalf("TestFilePatterns: %v", err)
	}
}

func TestMutationCoverageUsesTokenPathFilters(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	const stepID = "mutationsites-cov-token-filters"
	store := NewArtifactStore()
	if err := store.Write(stepID, "mutations.json", []byte(json), Provenance{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
	rawScopeWithoutExclude := mustNewQualityScope(t, "./...")
	out := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: rawScopeWithoutExclude,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		outputMode:   OutputModeVerbose,
	}
	if err := MutationCoverage(out, MinMutationCoverage(50)); err != nil {
		t.Fatalf("persisted token filters should exclude vendor despite raw scope: %v", err)
	}
}

// TestMutationCoverage_ScanPathUsesSameScopedInclusionAsSiteBudget exercises the stored
// mutations.json shape: [MutationCoverage] must agree with [gatecheck.CheckKillsReportSiteBudget]
// on which files count when [Exclude] applies (no duplicated path logic in this test).
func TestMutationCoverage_ScanPathUsesSameScopedInclusionAsSiteBudget(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"},{"status":"RUNNABLE"}]}
	]}`
	const stepID = "mutationsites-cov-align"
	store := NewArtifactStore()
	if err := store.Write(stepID, "mutations.json", []byte(json), Provenance{Tool: "test"}); err != nil {
		t.Fatal(err)
	}
	scope, err := NewQualityScope("./...", Exclude("vendor"))
	if err != nil {
		t.Fatal(err)
	}
	out := MutationScanOutput{
		store:        store,
		stepID:       stepID,
		qualityScope: scope,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		outputMode:   OutputModeVerbose,
	}
	if covErr := MutationCoverage(out, MinMutationCoverage(50)); covErr != nil {
		t.Fatalf("scan-path coverage with exclude: %v", covErr)
	}
	parsed, err := gatecheck.ParseMutationKillsReport(bytes.NewReader([]byte(json)))
	if err != nil {
		t.Fatalf("parse same JSON as store: %v", err)
	}
	if err := gatecheck.CheckKillsReportSiteBudget(parsed, 1, []string{"vendor"}, nil); err == nil {
		t.Fatal("expected site budget max 1 to fail: excluded report still has 2 sites in pkg/x.go")
	} else if !errors.Is(err, ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}

func TestMutationCoverageFromKills_PassAndFail(t *testing.T) {
	t.Parallel()
	const json = `{"mutations":[
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"RUNNABLE"},
		{"file":"a.go","package":"p","status":"NOT_COVERED"}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{stepID: "mk-1", qualityScope: scope, check: result.Check}
	if err := MutationCoverageFromKills(out, MinMutationCoverage(50)); err != nil {
		t.Fatalf("50%%: %v", err)
	}
	if err := MutationCoverageFromKills(out, MinMutationCoverage(66)); err != nil {
		t.Fatalf("66%%: %v", err)
	}
	highErr := MutationCoverageFromKills(out, MinMutationCoverage(67))
	if highErr == nil {
		t.Fatal("expected failure at 67%%")
	}
	if !errors.Is(highErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", highErr)
	}
}

func TestMutationCoverageFromKills_DisabledThresholdStillRequiresCompleteToken(t *testing.T) {
	t.Parallel()
	t.Run("invalidKillOutput", func(t *testing.T) {
		t.Parallel()
		err := MutationCoverageFromKills(MutationKillsOutput{}, MinMutationCoverage(0))
		if err == nil {
			t.Fatal("expected error for incomplete token even when threshold disabled")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
	t.Run("validKillOutput", func(t *testing.T) {
		t.Parallel()
		scope := mustNewQualityScope(t, "./...")
		out := MutationKillsOutput{
			stepID:       "mk-disabled",
			qualityScope: scope,
			check: &gatecheck.MutationKillsCheck{
				TotalRunnable:   2,
				TotalNotCovered: 1,
			},
		}
		if err := MutationCoverageFromKills(out, MinMutationCoverage(0)); err != nil {
			t.Fatalf("MinMutationCoverage(0) with valid token: %v", err)
		}
	})
}

func TestMutationCoverageFromKills_UsesEmbeddedCheckForThreshold(t *testing.T) {
	t.Parallel()
	// Boundary matches gatecheck coverage math on these totals alone (no JSON, no store).
	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{
		stepID:       "mk-embedded",
		qualityScope: scope,
		check: &gatecheck.MutationKillsCheck{
			TotalRunnable:   2,
			TotalNotCovered: 1,
			Files: []gatecheck.FileMutationStats{
				{File: "pkg/a.go", Runnable: 2, NotCovered: 1},
			},
		},
	}
	if err := MutationCoverageFromKills(out, MinMutationCoverage(66)); err != nil {
		t.Fatalf("66%%: %v", err)
	}
	failErr := MutationCoverageFromKills(out, MinMutationCoverage(67))
	if failErr == nil {
		t.Fatal("expected failure at 67%%")
	}
	if !errors.Is(failErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", failErr)
	}
}

func TestMutationCoverageFromKills_ExcludeScopeChangesCoverage(t *testing.T) {
	t.Parallel()
	// Same shape as [TestMutationSitesFromKills_ExcludeScopeIgnoresPath]: vendor dilutes
	// product-only coverage; excluding vendor flips a 50%% threshold.
	const json = `{"files":[
		{"file_name":"internal/vendor/v.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/x.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outAll := MutationKillsOutput{stepID: "mk-cov-all", qualityScope: scopeAll, check: result.Check}
	if covErr := MutationCoverageFromKills(outAll, MinMutationCoverage(50)); covErr == nil {
		t.Fatal("unscoped: expected failure at 50%")
	} else if !errors.Is(covErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", covErr)
	}
	scopeNoVendor, err := NewQualityScope("./...", Exclude("vendor"))
	if err != nil {
		t.Fatal(err)
	}
	outProd := MutationKillsOutput{
		stepID:       "mk-cov-prod",
		qualityScope: scopeNoVendor,
		pathFilters:  testMutationPathFilters([]string{"vendor"}, nil),
		check:        result.Check,
	}
	if covErr := MutationCoverageFromKills(outProd, MinMutationCoverage(50)); covErr != nil {
		t.Fatalf("exclude vendor: %v", covErr)
	}
}

func TestMutationCoverageFromKills_TestFilePatternsChangeCoverage(t *testing.T) {
	t.Parallel()
	const json = `{"files":[
		{"file_name":"pkg/a_test.go","package":"p","mutations":[
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},
			{"status":"NOT_COVERED"},{"status":"NOT_COVERED"},{"status":"NOT_COVERED"}
		]},
		{"file_name":"pkg/a.go","package":"p","mutations":[{"status":"RUNNABLE"}]}
	]}`
	result, err := gatecheck.MutationKills([]byte(json), 0)
	if err != nil {
		t.Fatal(err)
	}
	scopeAll, err := NewQualityScope("./...")
	if err != nil {
		t.Fatal(err)
	}
	outAll := MutationKillsOutput{stepID: "mk-cov-tfall", qualityScope: scopeAll, check: result.Check}
	if covErr := MutationCoverageFromKills(outAll, MinMutationCoverage(50)); covErr == nil {
		t.Fatal("unscoped: expected failure at 50%")
	} else if !errors.Is(covErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", covErr)
	}
	scopeTestSkip, err := NewQualityScope("./...", TestFilePatterns("*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	outSkip := MutationKillsOutput{
		stepID:       "mk-cov-tfskip",
		qualityScope: scopeTestSkip,
		pathFilters:  testMutationPathFilters(nil, []string{"*_test.go"}),
		check:        result.Check,
	}
	if covErr := MutationCoverageFromKills(outSkip, MinMutationCoverage(50)); covErr != nil {
		t.Fatalf("TestFilePatterns: %v", covErr)
	}
}

func TestMutationCoverageFromKills_RejectsIncompleteOutput(t *testing.T) {
	t.Parallel()
	t.Run("missingCheck", func(t *testing.T) {
		t.Parallel()
		out := MutationKillsOutput{stepID: "mk-incomplete"}
		err := MutationCoverageFromKills(out, MinMutationCoverage(1))
		if err == nil {
			t.Fatal("expected error for missing check")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
	t.Run("emptyStepID", func(t *testing.T) {
		t.Parallel()
		out := MutationKillsOutput{check: &gatecheck.MutationKillsCheck{}}
		err := MutationCoverageFromKills(out, MinMutationCoverage(1))
		if err == nil {
			t.Fatal("expected error for empty stepID")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
	t.Run("missingQualityScope", func(t *testing.T) {
		t.Parallel()
		out := MutationKillsOutput{stepID: "mk-noscope", check: &gatecheck.MutationKillsCheck{}}
		err := MutationCoverageFromKills(out, MinMutationCoverage(1))
		if err == nil {
			t.Fatal("expected error for missing quality scope")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("expected ErrMissingValue, got %v", err)
		}
	})
}

func TestMutationCoverageFromKillsFailureUsesKillOutputModeForDiagnostics(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{
		stepID:       "mk-embedded",
		qualityScope: scope,
		check: &gatecheck.MutationKillsCheck{
			TotalRunnable:   2,
			TotalNotCovered: 1,
		},
	}
	t.Run("silent_token_wraps_diagnostic", func(t *testing.T) {
		t.Parallel()
		silent := out
		silent.outputMode = OutputModeAgent
		err := MutationCoverageFromKills(silent, MinMutationCoverage(67))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if !errors.As(err, &de) {
			t.Fatalf("silent outputMode: want *DiagnosticError, got %T: %v", err, err)
		}
		if !errors.Is(err, ErrMutationCoverageFailed) {
			t.Fatalf("errors.Is must still reach ErrMutationCoverageFailed, got %v", err)
		}
	})
	t.Run("zero_mode_stays_verbose_chain", func(t *testing.T) {
		t.Parallel()
		err := MutationCoverageFromKills(out, MinMutationCoverage(67))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if errors.As(err, &de) {
			t.Fatalf("zero outputMode: expected raw verbose chain, got diagnostic %v", err)
		}
		if !errors.Is(err, ErrMutationCoverageFailed) {
			t.Fatalf("expected ErrMutationCoverageFailed, got %v", err)
		}
	})
}

// TestMutationCoverageVerboseReturnsRawErrorChain verifies that verbose mode returns the
// raw error chain without DiagnosticError wrapping, preserving errors.Is behavior.
func TestMutationCoverageVerboseReturnsRawErrorChain(t *testing.T) {
	t.Parallel()

	scope := mustNewQualityScope(t, "./...")
	out := MutationKillsOutput{
		stepID:       "mk-verbose-raw",
		qualityScope: scope,
		check: &gatecheck.MutationKillsCheck{
			TotalRunnable:   2,
			TotalNotCovered: 1,
			Files: []gatecheck.FileMutationStats{
				{File: "pkg/a.go", Runnable: 2, NotCovered: 1},
			},
		},
	}

	// Verbose mode (explicit).
	out.outputMode = OutputModeVerbose

	err := MutationCoverageFromKills(out, MinMutationCoverage(67))
	if err == nil {
		t.Fatal("expected error at 67%")
	}

	// Verify it's NOT a DiagnosticError.
	var de *DiagnosticError
	if errors.As(err, &de) {
		t.Fatalf("expected raw error chain in verbose mode, got *DiagnosticError: %v", err)
	}

	// Verify errors.Is still reaches the sentinel.
	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrMutationCoverageFailed) must be true, got %v", err)
	}
}

// TestMutationCoverageStructuredResultContainsWorstFileRows verifies that the structured
// MutationCoverageResult contains the correct worst-file rows for curated diagnostics.
func TestMutationCoverageStructuredResultContainsWorstFileRows(t *testing.T) {
	t.Parallel()

	snap := buildMutationCoverageTestSnapshot()
	result, err := gatecheck.CheckMutationCoverageOnMetricsSnapshot(snap, 80, nil, nil)
	if err == nil {
		t.Fatal("expected failure at 80%")
	}

	verifyMutationCoverageResult(t, result)
}

func buildMutationCoverageTestSnapshot() gatecheck.MutationMetricsSnapshot {
	return gatecheck.MutationMetricsSnapshot{
		Files: []gatecheck.MutationFileMetrics{
			{File: "pkg/good.go", Killed: 5, Lived: 0, NotCovered: 0, TimedOut: 0, NotViable: 0, Runnable: 0},
			{File: "pkg/bad.go", Killed: 1, Lived: 0, NotCovered: 2, TimedOut: 0, NotViable: 0, Runnable: 0},
			{File: "pkg/ugly.go", Killed: 0, Lived: 0, NotCovered: 3, TimedOut: 0, NotViable: 0, Runnable: 0},
		},
	}
}
