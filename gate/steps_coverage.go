package gate

import (
	"context"
	"errors"
	"fmt"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	"github.com/hotchkj/mage-gate/internal/harness"
)

// Coverage checks thresholds using artifacts from [CoveredTest] ([CoveredTestOutput] token required).
//
//nolint:gocritic // Opaque value token
func Coverage(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	coveredOutput CoveredTestOutput,
	minPercent CoverageThreshold,
) (out CoverageOutput, err error) {
	if rootErr := validateRootAndStoreDeps(root, runner, fileOps, store); rootErr != nil {
		return CoverageOutput{}, rootErr
	}
	emitStepStart(runner, stepLineCoverage, coveredOutput.qualifier)
	if checkErr := validateMinPercent(minPercent); checkErr != nil {
		return CoverageOutput{}, checkErr
	}
	if checkErr := validateCoveredTestToken(&coveredOutput); checkErr != nil {
		return CoverageOutput{}, checkErr
	}
	if checkErr := requireUpstreamArtifact(store, coveredOutput.stepID, "coverage.out"); checkErr != nil {
		return CoverageOutput{}, checkErr
	}
	id := nextID("coverage")
	out = CoverageOutput{stepID: id, qualityScope: coveredOutput.qualityScope, qualifier: coveredOutput.qualifier}
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", qualityScopePackages(coveredOutput.qualityScope), runner, fileOps, store, id,
	)
	if err != nil {
		return CoverageOutput{}, fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("coverage", runner, harn.Cleanup())) }()
	commandScope := qualityScopeCommandScope(coveredOutput.qualityScope, nil)
	err = wrapCoverageError(runner, harn.StepCoverage(
		ctx,
		minPercent.minPercent,
		coveredOutput.stepID,
		commandScope,
	))
	if err != nil {
		return CoverageOutput{}, err
	}
	return out, nil
}

// wrapCoverageError handles coverage errors with structured diagnostics.
// In silent mode, it extracts structured data from CoverageFailure for better diagnostics.
// For non-typed errors, it falls back to stepDiagnostic.
func wrapCoverageError(runner CommandRunner, err error) error {
	if err == nil {
		return nil
	}

	if RunnerOutputMode(runner) != OutputModeAgent {
		return err
	}

	var covFail *harness.CoverageFailure
	if errors.As(err, &covFail) {
		diagErr := buildCoverageDiagnosticFromResult(covFail)
		emitDiagnosticIfPossible(runnerAsStepDisplay(runner), diagErr)
		return diagErr
	}

	diagErr := stepDiagnostic("coverage", err)
	emitDiagnosticIfPossible(runnerAsStepDisplay(runner), diagErr)
	return diagErr
}

func buildCoverageDiagnosticFromResult(covFail *harness.CoverageFailure) error {
	result := covFail.Result()
	fix, hint := sentinelDiagnostic("coverage", covFail)
	toolOutput := gatecheck.FormatCoverageDiagnosticRows(&result, gatecheck.MaxWorstFileRows)

	return cmdrunner.NewDiagnosticError(
		"coverage",
		fmt.Sprintf("coverage %.1f%% (required >= %.1f%%)", result.TotalCoverage, result.MinCoverage),
		fix,
		hint,
		&cmdrunner.DiagnosticOptions{Cause: covFail, ToolOutput: toolOutput},
	)
}
