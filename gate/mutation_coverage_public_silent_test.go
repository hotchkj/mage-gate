// Vision: Silent-mode mutation coverage diagnostics build structured DiagnosticError from metrics.
package gate

import (
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func TestMutationCoverageSilentBuildsStructuredDiagnostic(t *testing.T) {
	t.Parallel()

	out := buildMutationCoverageSilentTestOutput(t)
	err := MutationCoverageFromKills(out, MinMutationCoverage(67))
	if err == nil {
		t.Fatal("expected error at 67%")
	}

	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DiagnosticError, got %T: %v", err, err)
	}

	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("errors.Is(err, ErrMutationCoverageFailed) must be true, got %v", err)
	}

	snap, snapErr := out.metricsSnapshotForCoverage()
	if snapErr != nil {
		t.Fatalf("metricsSnapshotForCoverage: %v", snapErr)
	}
	result, covErr := gatecheck.CheckMutationCoverageOnMetricsSnapshot(
		snap, 67, out.pathFilters.excludeSegments, out.pathFilters.testFilePatterns,
	)
	if covErr == nil {
		t.Fatal("expected mutation coverage threshold failure")
	}
	if !errors.Is(covErr, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", covErr)
	}
	if result == nil {
		t.Fatal("expected structured result on threshold failure")
	}
	verifyMutationCoverageDiagnosticWithMsg(t, de, testFixMsg, testHintMsg, result)
}

func buildMutationCoverageSilentTestOutput(t *testing.T) MutationKillsOutput {
	scope := mustNewQualityScope(t, "./...")
	return MutationKillsOutput{
		stepID:       "mk-silent-struct",
		qualityScope: scope,
		pathFilters:  testMutationPathFilters(nil, nil),
		check: &gatecheck.MutationKillsCheck{
			TotalRunnable:   2,
			TotalNotCovered: 1,
			Files: []gatecheck.FileMutationStats{
				{File: "pkg/a.go", Runnable: 2, NotCovered: 1},
			},
		},
		outputMode: OutputModeAgent,
	}
}
