package gate

import (
	"context"
	"errors"
	"fmt"

	"github.com/hotchkj/mage-gate/internal/harness"
)

// Crap scores CRAP using [CoverageOutput] from [Coverage].
//
//nolint:gocritic // Opaque value token
func Crap(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	covOutput CoverageOutput,
	inventory QualityScopeInventoryOutput,
	maxScore CrapThreshold,
	gocycloSpec GocycloToolValue,
	opts ...CrapOption,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineCrap, covOutput.qualifier)
	cfg := defaultCrapConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := crapValidatePrerequisites(
		runner, resolver, fileOps, store, &covOutput, maxScore, gocycloSpec,
	); checkErr != nil {
		return checkErr
	}
	if checkErr := requireQualityScopeInventory(&inventory, store, root, covOutput.qualityScope); checkErr != nil {
		return checkErr
	}
	id := nextID("crap")
	pkgs := qualityScopePackages(covOutput.qualityScope)
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", pkgs, runner, fileOps, store, id,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("crap", runner, harn.Cleanup())) }()
	commandScope := qualityScopeCommandScope(covOutput.qualityScope, &inventory)
	return wrapStepError(
		"crap",
		runner,
		harn.StepCrap(
			ctx,
			maxScore.maxScore,
			covOutput.stepID,
			commandScope,
			gocycloSpec.spec,
			cfg.crapArgs,
		),
	)
}

// Duration checks per-test elapsed times from test-events.jsonl.
// The input is [TestOutput] from [Test] or [CoveredTestOutput.TestRun].
// Every test completion event in that run is checked; quality-scope excludes do not apply.
// Package build time, -coverpkg instrumentation, and package-level go test wall-clock are not measured.
// Validates options and token before opening artifacts so misconfiguration fails without a successful upstream test.
//
//nolint:gocritic // Opaque value token
func Duration(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	testOutput TestOutput,
	maxSeconds DurationThreshold,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineDuration, testOutput.qualifier)
	if checkErr := validateMaxSeconds(maxSeconds); checkErr != nil {
		return checkErr
	}
	if checkErr := requireStoreDeps(runner, fileOps, store); checkErr != nil {
		return checkErr
	}
	if checkErr := validateTestOutputToken(testOutput); checkErr != nil {
		return checkErr
	}
	if checkErr := requireUpstreamArtifact(store, testOutput.stepID, "test-events.jsonl"); checkErr != nil {
		return checkErr
	}
	id := nextID("duration")
	harn, err := harness.NewStepRunner(
		gateRoot(root), "", testOutput.scope.Packages(), runner, fileOps, store, id,
	)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("duration", runner, harn.Cleanup())) }()
	return wrapStepError(
		"duration",
		runner,
		harn.StepDuration(ctx, maxSeconds.maxSeconds, testOutput.stepID),
	)
}
