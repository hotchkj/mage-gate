// Vision: Mutation entrypoints split from steps.go—same harness contracts with a smaller, consumer-oriented surface.
package gate

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	"github.com/hotchkj/mage-gate/internal/harness"
)

// mutationRunnerScanMaxSites skips per-file site enforcement in the harness so [MutationRunner.Scan]
// can pair with a separate sites check; [MutationSites] passes the real cap instead.
const (
	mutationRunnerScanMaxSites = math.MaxInt
)

// Dry-run scan operation identity: [MutationRunner.Scan] uses step prefix mutationscan.
const scanOpMutationscan = "mutationscan"

// mutationMetricsSource lets coverage read metrics from either scan or kill outputs.
type mutationMetricsSource interface {
	metricsSnapshotForCoverage() (gatecheck.MutationMetricsSnapshot, error)
}

type mutationPathFilters struct {
	excludeSegments  []string
	testFilePatterns []string
}

func mutationPathFiltersFromCommandScope(commandScope *gatecheck.QualityScopeCommandScope) mutationPathFilters {
	excludeSegments, testFilePatterns := commandScope.ThresholdPathFilters()
	return mutationPathFilters{
		excludeSegments:  excludeSegments,
		testFilePatterns: testFilePatterns,
	}
}

func (f mutationPathFilters) thresholdPathFilters() (excludeSegments, testFilePatterns []string) {
	return append([]string(nil), f.excludeSegments...),
		append([]string(nil), f.testFilePatterns...)
}

// retrieveMutationKillsCheck reads the mutations.json artifact from the store and parses it into a MutationKillsCheck.
func retrieveMutationKillsCheck(store *ArtifactStore, stepID string) (*gatecheck.MutationKillsCheck, error) {
	data, err := store.Read(stepID, "mutations.json")
	if err != nil {
		return nil, fmt.Errorf("read mutations artifact: %w", err)
	}

	result, err := gatecheck.MutationKills(data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse mutations data: %w", err)
	}
	return result.Check, nil
}

// retrieveMutationMetricsSnapshot centralizes scan-artifact -> shared metrics snapshot conversion.
func retrieveMutationMetricsSnapshot(store *ArtifactStore, stepID string) (gatecheck.MutationMetricsSnapshot, error) {
	check, err := retrieveMutationKillsCheck(store, stepID)
	if err != nil {
		return gatecheck.MutationMetricsSnapshot{}, err
	}
	return gatecheck.SnapshotFromMutationKillsCheck(check), nil
}

// metricsSnapshotForCoverage implements [mutationMetricsSource] for a dry-run scan token.
func (o *MutationScanOutput) metricsSnapshotForCoverage() (gatecheck.MutationMetricsSnapshot, error) {
	if o == nil {
		return gatecheck.MutationMetricsSnapshot{}, fmt.Errorf("%w: MutationScanOutput is nil", ErrMissingValue)
	}
	if o.stepID == "" {
		return gatecheck.MutationMetricsSnapshot{}, fmt.Errorf("%w: MutationScanOutput stepID is empty", ErrMissingValue)
	}
	if o.store == nil {
		return gatecheck.MutationMetricsSnapshot{}, fmt.Errorf("%w: Store", ErrNilDependency)
	}
	return retrieveMutationMetricsSnapshot(o.store, o.stepID)
}

// metricsSnapshotForCoverage keeps full-run coverage checks on embedded kill metrics.
func (o *MutationKillsOutput) metricsSnapshotForCoverage() (gatecheck.MutationMetricsSnapshot, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return gatecheck.MutationMetricsSnapshot{}, err
	}
	return gatecheck.SnapshotFromMutationKillsCheck(o.check), nil
}

func runMutationScan(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	qualityScope QualityScope,
	inventory *QualityScopeInventoryOutput,
	mutationSitesMax int,
	scanOpStepName string,
	gremlinsSpec GremlinsToolValue,
	opts ...MutationOption,
) (out MutationScanOutput, err error) {
	if scanOpStepName == scanOpMutationscan {
		emitStepStart(runner, stepLineMutationScan, "")
	}
	cfg := defaultMutationConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := rejectBuildTagArgs(scanOpStepName, cfg.mutationArgs); checkErr != nil {
		return MutationScanOutput{}, checkErr
	}
	if checkErr := requireQualityScopeInventory(inventory, store, root, qualityScope); checkErr != nil {
		return MutationScanOutput{}, checkErr
	}
	pkgs := qualityScopePackages(qualityScope)
	id := nextID(scanOpStepName)
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", pkgs, runner, fileOps, store, id,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return MutationScanOutput{}, fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup(scanOpStepName, runner, harn.Cleanup())) }()
	commandScope := qualityScopeCommandScope(qualityScope, inventory)
	stepErr := harn.StepMutationScan(
		ctx,
		mutationSitesMax,
		commandScope,
		gremlinsSpec.spec,
		cfg.mutationArgs,
	)
	if stepErr != nil {
		return MutationScanOutput{}, wrapStepError(scanOpStepName, runner, stepErr)
	}
	return MutationScanOutput{
		store:        store,
		stepID:       id,
		qualityScope: qualityScope,
		pathFilters:  mutationPathFiltersFromCommandScope(commandScope),
		outputMode:   RunnerOutputMode(runner),
		display:      runnerAsStepDisplay(runner),
	}, nil
}

func mutationKillsHarness(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	qualityScope QualityScope,
	inventory *QualityScopeInventoryOutput,
	minKillRate MinKillRateThreshold,
	gremlinsSpec GremlinsToolValue,
	mutationArgs []string,
) (out MutationKillsOutput, err error) {
	emitStepStart(runner, stepLineMutationKills, "")
	if checkErr := rejectBuildTagArgs("mutationkills", mutationArgs); checkErr != nil {
		return MutationKillsOutput{}, checkErr
	}
	if checkErr := requireQualityScopeInventory(inventory, store, root, qualityScope); checkErr != nil {
		return MutationKillsOutput{}, checkErr
	}
	pkgs := qualityScopePackages(qualityScope)
	id := nextID("mutationkills")
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", pkgs, runner, fileOps, store, id,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return MutationKillsOutput{}, fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("mutationkills", runner, harn.Cleanup())) }()
	commandScope := qualityScopeCommandScope(qualityScope, inventory)
	stepErr := harn.StepMutationKills(
		ctx,
		0,
		commandScope,
		gremlinsSpec.spec,
		mutationArgs,
	)
	if stepErr != nil {
		return MutationKillsOutput{}, wrapStepError("mutationkills", runner, stepErr)
	}

	check, checkErr := retrieveMutationKillsCheck(store, id)
	if checkErr != nil {
		return MutationKillsOutput{}, wrapStepError("mutationkills", runner, checkErr)
	}

	out = MutationKillsOutput{
		stepID:       id,
		qualityScope: qualityScope,
		pathFilters:  mutationPathFiltersFromCommandScope(commandScope),
		check:        check,
		outputMode:   RunnerOutputMode(runner),
		display:      runnerAsStepDisplay(runner),
	}
	if rateErr := MutationKillRate(out, minKillRate); rateErr != nil {
		return out, rateErr
	}
	return out, nil
}

// MutationSites enforces the per-file mutation site budget against mutations.json already stored
// by [MutationRunner.Scan], correlated via scanOut. It does not run gremlins. The [ArtifactStore]
// bound in scanOut (by the scan producer) holds the durable report; scanOut also carries
// correlation identity and [QualityScope].
//
//nolint:gocritic // Opaque value token
func MutationSites(scanOut MutationScanOutput, maxSites MutationSitesThreshold) error {
	emitStepStartFromToken(scanOut.display, stepLineMutationSites)
	if err := validateMaxSites(maxSites); err != nil {
		return err
	}
	store := scanOut.store
	if store == nil {
		return fmt.Errorf("%w: Store", ErrNilDependency)
	}
	mode := scanOut.outputMode
	stepID, err := scanOut.StepID()
	if err != nil {
		return err
	}
	check, rerr := retrieveMutationKillsCheck(store, stepID)
	if rerr != nil {
		return wrapStepErrorWithMode("mutationsites", mode, rerr, scanOut.display)
	}
	if _, qerr := scanOut.QualityScope(); qerr != nil {
		return wrapStepErrorWithMode("mutationsites", mode, qerr, scanOut.display)
	}
	excludeSegs, testPatterns := scanOut.pathFilters.thresholdPathFilters()
	if cerr := gatecheck.CheckKillsReportSiteBudget(
		check,
		maxSites.maxSites,
		excludeSegs,
		testPatterns,
	); cerr != nil {
		return wrapStepErrorWithMode("mutationsites", mode, cerr, scanOut.display)
	}
	return nil
}

// MutationCoverageFromKills enforces mutation test-profile coverage using the parsed gremlins
// report in killOut (for example from [MutationKills]), without reading the artifact store.
// MinMutationCoverage(0) skips the coverage threshold only; killOut must still be a complete
// scoped token, matching other step output consumers.
// [QualityScope] on the token filters gremlins file rows with [Exclude] and [TestFilePatterns]
// the same way as [MutationSitesFromKills] (per-file tallies, not a second JSON parse).
// Error shaping follows the output mode recorded on killOut from the kill run; a zero or
// unknown mode behaves like [OutputModeVerbose], matching other gate step diagnostics.
//
//nolint:gocritic // Opaque value token
func MutationCoverageFromKills(killOut MutationKillsOutput, minCoverage MutationCoverageThreshold) error {
	emitStepStartFromToken(killOut.display, stepLineMutationCoverage)
	if err := validateMinMutationCoverage(minCoverage); err != nil {
		return err
	}
	if err := (&killOut).validateScopedMetricsAccess(); err != nil {
		return err
	}
	if minCoverage.minPercent <= 0 {
		return nil
	}
	excludeSegs, testPatterns := killOut.pathFilters.thresholdPathFilters()
	return applyMutationCoverageFromSource(
		killOut.outputMode,
		killOut.display,
		&killOut,
		minCoverage.minPercent,
		excludeSegs,
		testPatterns,
	)
}

// MutationKillRate enforces the minimum mutation kill rate using metrics already
// embedded in killOut (for example from [MutationRunner.Kill] or [MutationKills]).
// It does not re-read artifacts or re-parse JSON; it uses EvaluateMutationKills on the embedded check.
// [MinKillRate(0)] disables the check.
// Silent output mode surfaces a DiagnosticError built from MutationKillsResult structured fields (summary
// Cause, survivor rows, timeouts); verbose and unknown modes return the raw formatted error chain
// produced by FormatMutationKillsReport.
//
//nolint:gocritic // Opaque value token
func MutationKillRate(killOut MutationKillsOutput, minKillRate MinKillRateThreshold) error {
	if err := validateMinKillRate(minKillRate); err != nil {
		return err
	}
	if err := (&killOut).validateMetricsAccess(); err != nil {
		return err
	}
	result := gatecheck.EvaluateMutationKills(killOut.check, minKillRate.minPercent)
	if result.Passed {
		return nil
	}

	if killOut.outputMode == OutputModeAgent {
		diagErr := buildMutationKillsDiagnosticFromResult(result)
		emitDiagnosticIfPossible(killOut.display, diagErr)
		return diagErr
	}

	inner := fmt.Errorf("%w: %s", ErrMutationKillsFailed, gatecheck.FormatMutationKillsReport(result))
	return wrapStepErrorWithMode("mutationkills", killOut.outputMode, inner, killOut.display)
}

// buildMutationKillsDiagnosticFromResult builds DiagnosticError from MutationKillsResult for silent mode;
// Cause chains to ErrMutationKillsFailed via ThresholdError. ToolOutput is formatted detail sections only,
// truncated for size — not passed through filterFallbackToolOutput (threshold path does not scrape stderr here).
func buildMutationKillsDiagnosticFromResult(result gatecheck.MutationKillsResult) error {
	if result.Check == nil {
		return fmt.Errorf("%w: no mutation kill report data", ErrMutationKillsFailed)
	}
	if result.ThresholdError == nil {
		return fmt.Errorf("%w", ErrMutationKillsFailed)
	}

	fix, hint := sentinelDiagnostic("mutationkills", result.ThresholdError)
	toolOut := gatecheck.FormatMutationKillsDetailSections(result.Check)
	opts := &cmdrunner.DiagnosticOptions{
		Cause: result.ThresholdError,
	}
	if toolOut != "" {
		opts.ToolOutput = truncateToolOutput(toolOut)
	}
	return cmdrunner.NewDiagnosticError(
		"mutationkills",
		result.ThresholdError.Error(),
		fix,
		hint,
		opts,
	)
}

// MutationKills runs full gremlins (kill mode), validates kill rate, returns [MutationKillsOutput].
// On-demand—not part of the default Gate() path.
//
//nolint:gocritic // Opaque value token
func MutationKills(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	qualityScope QualityScope,
	inventory QualityScopeInventoryOutput,
	minKillRate MinKillRateThreshold,
	gremlinsSpec GremlinsToolValue,
	opts ...MutationOption,
) (out MutationKillsOutput, err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return MutationKillsOutput{}, rootErr
	}
	cfg := defaultMutationConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := mutationValidateBeforeHarnessForKills(
		qualityScope, minKillRate, gremlinsSpec, runner, resolver, fileOps, store,
	); checkErr != nil {
		return MutationKillsOutput{}, checkErr
	}
	return mutationKillsHarness(
		ctx, runner, resolver, store, fileOps, root, qualityScope, &inventory,
		minKillRate, gremlinsSpec, cfg.mutationArgs,
	)
}
