// Vision: Mutation coverage and kills-from-kills checks split from mutation_kills_public.go by theme.
package gate

import (
	"fmt"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// MutationSitesFromKills enforces the per-file mutation site budget from the embedded
// [MutationKillsCheck] in killOut (for example from [MutationKills]), without reading the
// artifact store. The same [QualityScope] on the token filters gremlins file paths with
// [Exclude] and [TestFilePatterns] like the stored-report path, using the check's per-file
// tallies (not re-parsing JSON). Failures follow the output mode recorded on killOut; a zero
// or unknown mode behaves like [OutputModeVerbose], matching other gate step diagnostics.
//
//nolint:gocritic // Opaque value token
func MutationSitesFromKills(killOut MutationKillsOutput, maxSites MutationSitesThreshold) error {
	emitStepStartFromToken(killOut.display, stepLineMutationSites)
	if err := validateMaxSites(maxSites); err != nil {
		return err
	}
	if err := (&killOut).validateScopedMetricsAccess(); err != nil {
		return err
	}
	excludeSegs, testPatterns := killOut.pathFilters.thresholdPathFilters()
	if cerr := gatecheck.CheckKillsReportSiteBudget(
		killOut.check,
		maxSites.maxSites,
		excludeSegs,
		testPatterns,
	); cerr != nil {
		return wrapStepErrorWithMode("mutationsites", killOut.outputMode, cerr, killOut.display)
	}
	return nil
}

// applyMutationCoverageFromSource enforces mutation coverage using metrics from a source.
// It branches on outputMode: for silent mode, it builds a structured DiagnosticError from
// the structured MutationCoverageResult; for verbose/unknown mode, it returns the raw error.
func applyMutationCoverageFromSource(
	outputMode OutputMode,
	display stepDisplay,
	src mutationMetricsSource,
	minPercent int,
	excludeSegments []string,
	testFilePatterns []string,
) error {
	snap, serr := src.metricsSnapshotForCoverage()
	if serr != nil {
		return wrapStepErrorWithMode("mutationcoverage", outputMode, serr, display)
	}
	result, cerr := gatecheck.CheckMutationCoverageOnMetricsSnapshot(
		snap, minPercent, excludeSegments, testFilePatterns,
	)
	if cerr == nil {
		return nil
	}

	if outputMode == OutputModeAgent {
		diagErr := buildMutationCoverageDiagnosticFromResult(result)
		emitDiagnosticIfPossible(display, diagErr)
		return diagErr
	}

	return wrapStepErrorWithMode("mutationcoverage", outputMode, cerr, display)
}

// buildMutationCoverageDiagnosticFromResult builds a structured DiagnosticError from the
// MutationCoverageResult for silent mode diagnostics. It constructs ERROR/Fix/Hint directly
// from the structured data without text parsing.
func buildMutationCoverageDiagnosticFromResult(result *gatecheck.MutationCoverageResult) error {
	if result == nil {
		return fmt.Errorf("%w: mutation coverage check returned nil result", ErrMutationCoverageFailed)
	}

	worstFileRows := gatecheck.FormatMutationCoverageResultRows(result, gatecheck.MaxWorstFileRows)

	fix := "expand the go test coverage profile (coverpkg) so more mutation points are considered covered"
	hint := "NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages"

	opts := &cmdrunner.DiagnosticOptions{
		Cause: result.Summary.ThresholdError,
	}

	if worstFileRows != "" {
		opts.ToolOutput = worstFileRows
	}

	var msg string
	if result.Summary.ThresholdError != nil {
		msg = result.Summary.ThresholdError.Error()
	} else {
		msg = fmt.Sprintf(
			"mutation coverage %.1f%% below threshold %d%% (%d of %d mutants covered; %d not covered by test profile)",
			result.Summary.Percent, result.Summary.MinPercent, result.Summary.Covered,
			result.Summary.Total, result.Summary.NotCovered,
		)
	}

	return cmdrunner.NewDiagnosticError(
		"mutationcoverage",
		msg,
		fix,
		hint,
		opts,
	)
}

// MutationCoverage checks gremlins dry-run / scan metrics in stored mutations.json against
// a minimum share of mutants with test profile coverage. MinMutationCoverage(0) disables
// the check. The [ArtifactStore] is taken from the scan token (set by the scan producer).
// [QualityScope] on the token filters gremlins file rows with [Exclude] and
// [TestFilePatterns] the same way as [MutationCoverageFromKills].
//
//nolint:gocritic // Opaque value token
func MutationCoverage(
	scanOut MutationScanOutput,
	minCoverage MutationCoverageThreshold,
) error {
	emitStepStartFromToken(scanOut.display, stepLineMutationCoverage)
	if err := validateMinMutationCoverage(minCoverage); err != nil {
		return err
	}
	if scanOut.store == nil {
		return fmt.Errorf("%w: Store", ErrNilDependency)
	}
	if _, err := scanOut.StepID(); err != nil {
		return err
	}
	if _, qerr := scanOut.QualityScope(); qerr != nil {
		return qerr
	}
	if minCoverage.minPercent <= 0 {
		return nil
	}
	excludeSegs, testPatterns := scanOut.pathFilters.thresholdPathFilters()
	return applyMutationCoverageFromSource(
		scanOut.outputMode,
		scanOut.display,
		&scanOut,
		minCoverage.minPercent,
		excludeSegs,
		testPatterns,
	)
}
