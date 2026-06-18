// Vision: BDD execution/runtime helpers live here (not with runner options) to satisfy repo file-size limits.
//
//nolint:revive // file-length-limit is enforced globally, BDD execution helpers are intentionally grouped.
package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

func (s *scenarioState) resolverForTool(toolName, state, spec string) cmdrunner.ToolResolver {
	if state == toolStateUnprobeable {
		return gatetest.NewFakeToolResolverError("unprobeable local binary")
	}
	resolver := gatetest.NewFakeToolResolver()
	if state == toolStateMatching && spec != "" {
		resolver.SetLocalMatch(toolName, spec, true)
	}
	return resolver
}

func (s *scenarioState) resolverForStep(stepName string) cmdrunner.ToolResolver {
	// Check if this is a pinned tool step using the registry
	if !isPinnedToolStep(stepName) {
		return gatetest.NewFakeToolResolver()
	}

	probeName, err := s.pinnedToolProbeName(stepName)
	if err != nil {
		return gatetest.NewFakeToolResolver()
	}

	state, _ := s.localToolState(stepName)
	spec := s.expectedToolSpecForStep(stepName)
	return s.resolverForTool(probeName, state, spec)
}

// parseGateStepList parses gate step names: comma-separated lists reject empty segments;
// lists without commas use whitespace separation (e.g. "mutationsites mutationcoverage").
func parseGateStepList(list string) ([]string, error) {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil, errGateStepListEmpty
	}
	var parts []string
	if strings.IndexByte(list, ',') >= 0 {
		for _, seg := range strings.Split(list, ",") {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				return nil, fmt.Errorf("%w in %q", errGateStepListBadSegment, list)
			}
			parts = append(parts, seg)
		}
	} else {
		for _, seg := range strings.Fields(list) {
			parts = append(parts, strings.TrimSpace(seg))
		}
	}
	if len(parts) == 0 {
		return nil, errGateStepListEmpty
	}
	return parts, nil
}

func (s *scenarioState) runGateStepsFromList(list string) error {
	names, err := parseGateStepList(list)
	if err != nil {
		return err
	}
	return s.runGateSteps(names...)
}

func (s *scenarioState) newFakeDisplayForGateSteps(names []string) (
	fr *cmdtest.FakeRunner,
	display qg.CommandRunner,
	err error,
) {
	for _, stepName := range names {
		stepOpts := s.composeRunnerOptions(stepName)
		if vErr := cmdtest.ValidateUniqueResponseKeys(stepOpts...); vErr != nil {
			return nil, nil, fmt.Errorf("step %q: %w", stepName, vErr)
		}
	}

	allOpts := s.composeMultiStepOpts(names)
	// Shared FakeRunner across steps reuses command keys; MergeDuplicateKeys keeps first handler on overlap.
	fr = cmdtest.NewFakeRunner(
		append([]cmdtest.RunnerOption{cmdtest.MergeDuplicateKeys()}, allOpts...)...,
	)
	display, err = qg.NewDisplayRunner(
		fr,
		s.outputMode,
		s.output,
		s.output,
	)
	return fr, display, err
}

// runGateStepsDispatchLoop runs each gate step in order. After each successful dispatch it
// validates the command contract for that step (stage-local). On dispatch failure, runErr is
// the primary error; if the contract for that stage also fails, errors.Join preserves both.
func (s *scenarioState) runGateStepsDispatchLoop(
	names []string,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
	fr *cmdtest.FakeRunner,
) bool {
	for _, stepName := range names {
		callsBefore := len(fr.Calls())
		stepErr := s.dispatchStep(stepName, display, store, mem, scope, pkgScope)
		stepCalls := fr.Calls()[callsBefore:]
		if stepErr != nil {
			s.stepName = stepName
			s.recordedCalls = stepCalls
			var ve *qg.ValidationError
			if errors.As(stepErr, &ve) {
				s.runErr = stepErr
				return true
			}
			if contractErr := s.validateContractCalls(stepCalls, stepName); contractErr != nil {
				s.runErr = errors.Join(stepErr, contractErr)
			} else {
				s.runErr = stepErr
			}
			return true
		}
		s.stepsRan = append(s.stepsRan, stepName)
		if contractErr := s.validateContractCalls(stepCalls, stepName); contractErr != nil {
			s.runErr = contractErr
			s.stepName = stepName
			s.recordedCalls = stepCalls
			return true
		}
	}
	return false
}

// runGateSteps executes one or more gate steps in order using a single runner and fake wiring.
//
//nolint:gocyclo // orchestration: resolve scope, build runner, loop dispatch+contract, capture calls
func (s *scenarioState) runGateSteps(names ...string) error {
	if len(names) == 0 {
		return errNoGateSteps
	}
	resolved := append([]string(nil), names...)
	for i := range resolved {
		resolved[i] = strings.ToLower(strings.TrimSpace(resolved[i]))
	}

	s.output.Reset()
	s.stepsRan = nil
	s.recordedCalls = nil
	s.allDispatchedCalls = nil
	s.stepName = resolved[0]

	mem := s.ensureMem()
	store := s.ensureStore()
	if s.pkgScopePattern == "" || s.qualityScopePattern == "" {
		s.runErr = errScopesRequired
		return nil
	}
	scope, err := s.buildQualityScope()
	if err != nil {
		s.runErr = err
		return nil
	}
	s.qualityScope = scope
	pkgScope, err := qg.NewPackageScope(s.pkgScopePattern)
	if err != nil {
		s.runErr = err
		return nil
	}

	fr, display, err := s.newFakeDisplayForGateSteps(resolved)
	if err != nil {
		s.runErr = err
		return nil
	}

	if s.runGateStepsDispatchLoop(resolved, display, store, mem, scope, pkgScope, fr) {
		return nil
	}

	s.runErr = nil
	s.stepName = resolved[len(resolved)-1]
	if sc, ok := s.stepCallsMap[s.stepName]; ok {
		if len(sc) > 0 {
			s.recordedCalls = sc[len(sc)-1]
		}
	}
	return nil
}

// composeMultiStepOpts merges fake runner options for each step name. Caller must pass
// normalized names (see runGateSteps); composeRunnerOptions keys on those ids.
func (s *scenarioState) composeMultiStepOpts(steps []string) []cmdtest.RunnerOption {
	var opts []cmdtest.RunnerOption
	for _, stepName := range steps {
		opts = append(opts, s.composeRunnerOptions(stepName)...)
	}
	return opts
}

func (s *scenarioState) dispatchStep(
	name string,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
) error {
	ctx := context.Background()
	root := "."

	if err := s.dispatchSimpleStep(ctx, name, runner, mem, root, scope, pkgScope); err != nil || isSimpleStep(name) {
		return err
	}
	return s.dispatchArtifactStep(ctx, name, runner, store, mem, root, scope, pkgScope)
}

func isSimpleStep(name string) bool {
	switch name {
	case stepLint, stepFormat, stepCompile, stepVet, stepDeadcode, stepMarkdownlint:
		return true
	default:
		return false
	}
}

func (s *scenarioState) dispatchSimpleStep(
	ctx context.Context,
	name string,
	runner qg.CommandRunner,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
) error {
	_ = scope
	switch name {
	case stepLint:
		return s.runLintStep(ctx, runner, mem, root, pkgScope)
	case stepFormat:
		return s.runFormatStep(ctx, runner, mem, root, pkgScope)
	case stepCompile:
		return qg.Compile(ctx, runner, mem, root, pkgScope, s.compileOptions()...)
	case stepVet:
		return qg.Vet(ctx, runner, mem, root, pkgScope, s.vetOptions()...)
	case stepDeadcode:
		return s.runDeadcodeStep(ctx, runner, mem, root, pkgScope)
	case stepMarkdownlint:
		return s.runMarkdownlintStep(ctx, runner, mem, root)
	default:
		return nil
	}
}

func (s *scenarioState) runLintStep(
	ctx context.Context,
	runner qg.CommandRunner,
	mem qg.FileOps,
	root string,
	pkgScope qg.PackageScope,
) error {
	lt, err := s.lintToolchainForStep(stepLint)
	if err != nil {
		return err
	}
	return qg.Lint(
		ctx,
		runner,
		s.resolverForStep(stepLint),
		mem,
		root,
		pkgScope,
		lt,
	)
}

func (s *scenarioState) runFormatStep(
	ctx context.Context,
	runner qg.CommandRunner,
	mem qg.FileOps,
	root string,
	pkgScope qg.PackageScope,
) error {
	lt, err := s.lintToolchainForStep(stepFormat)
	if err != nil {
		return err
	}
	return qg.Format(
		ctx,
		runner,
		s.resolverForStep(stepFormat),
		mem,
		root,
		pkgScope,
		lt,
	)
}

func (s *scenarioState) runDeadcodeStep(
	ctx context.Context,
	runner qg.CommandRunner,
	mem qg.FileOps,
	root string,
	pkgScope qg.PackageScope,
) error {
	toolSpec, hasSpec := s.deadcodeToolValue()
	if !hasSpec {
		toolSpec = qg.DeadcodeToolValue{}
	}
	return qg.Deadcode(
		ctx,
		runner,
		s.resolverForStep(stepDeadcode),
		mem,
		root,
		pkgScope,
		toolSpec,
		s.deadcodeOptions()...,
	)
}

func (s *scenarioState) runMarkdownlintStep(
	ctx context.Context,
	runner qg.CommandRunner,
	mem qg.FileOps,
	root string,
) error {
	toolSpec, hasSpec := s.markdownlintToolValue()
	if !hasSpec {
		toolSpec = qg.MarkdownLintToolValue{}
	}
	return qg.MarkdownLint(
		ctx,
		runner,
		s.resolverForStep(stepMarkdownlint),
		mem,
		root,
		toolSpec,
		s.markdownlintOptions()...,
	)
}

func (s *scenarioState) dispatchArtifactStep(
	ctx context.Context,
	name string,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
) error {
	if handled, err := s.dispatchMutationArtifactStep(ctx, name, runner, store, mem, root, scope); handled {
		return err
	}
	switch name {
	case stepTest:
		return s.runTest(ctx, runner, store, mem, root, pkgScope)
	case stepQualityInventory:
		return s.runQualityScopeInventory(ctx, runner, store, mem, root, scope)
	case stepCoveredTest:
		return s.runCoveredTest(ctx, runner, store, mem, root, scope, pkgScope)
	case stepCoverage:
		return s.runCoverage(ctx, runner, store, mem, root)
	case stepCrap:
		return s.runCrap(ctx, runner, store, mem, root)
	case stepDuration:
		return s.runDuration(ctx, runner, store, mem, root)
	default:
		return fmt.Errorf("%w: %q", errUnsupportedStep, name)
	}
}

func (s *scenarioState) dispatchMutationArtifactStep(
	ctx context.Context,
	name string,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
) (bool, error) {
	switch name {
	case stepMutationSites:
		return true, s.dispatchMutationSitesStep()
	case stepMutationScan:
		resolver := s.resolverForStep(name)
		return true, s.runMutationScanStep(ctx, runner, resolver, store, mem, root, scope)
	case stepMutationCoverage:
		return true, s.dispatchMutationCoverageStep()
	case stepMutationKills:
		resolver := s.resolverForStep(name)
		return true, s.runMutationKills(ctx, runner, resolver, store, mem, root, scope)
	default:
		return false, nil
	}
}

func (s *scenarioState) runQualityScopeInventory(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
) error {
	out, err := qg.QualityScopeInventory(ctx, display, store, mem, root, scope)
	if err != nil {
		return err
	}
	s.stepOpts["qualityScopeInventoryOutput"] = out
	return nil
}

func (s *scenarioState) runTest(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	pkgScope qg.PackageScope,
) error {
	testOut, err := qg.Test(ctx, display, store, mem, root, pkgScope, s.testOptions()...)
	if err != nil {
		return err
	}
	s.stepOpts["testOutput"] = testOut
	return nil
}

func (s *scenarioState) runCoveredTest(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
) error {
	inv, _ := s.priorQualityScopeInventoryOutput()
	unitCov, err := qg.CoveredTest(ctx, display, store, mem, root, pkgScope, scope, inv, s.testOptions()...)
	if err != nil {
		return err
	}
	s.stepOpts["coveredTestOutput"] = unitCov
	testOut, trErr := unitCov.TestRun()
	if trErr != nil {
		return trErr
	}
	s.stepOpts["testOutput"] = testOut
	return nil
}

func (s *scenarioState) runCoverage(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
) error {
	if _, ok := s.minPercent(); !ok {
		_, err := qg.Coverage(
			ctx,
			display,
			store,
			mem,
			root,
			qg.CoveredTestOutput{},
			qg.CoverageThreshold{},
		)
		return err
	}

	covToken, ok := s.priorCoveredTestOutput()
	if !ok {
		threshold := s.coverageThreshold()
		_, err := qg.Coverage(ctx, display, store, mem, root, qg.CoveredTestOutput{}, threshold)
		return err
	}
	threshold := s.coverageThreshold()
	covOut, err := qg.Coverage(ctx, display, store, mem, root, covToken, threshold)
	if err != nil {
		return err
	}
	s.stepOpts["coverageOutput"] = covOut
	return nil
}

func (s *scenarioState) priorTestOutput() (qg.TestOutput, bool) {
	out, ok := s.stepOpts["testOutput"].(qg.TestOutput)
	return out, ok
}

func (s *scenarioState) priorCoveredTestOutput() (qg.CoveredTestOutput, bool) {
	out, ok := s.stepOpts["coveredTestOutput"].(qg.CoveredTestOutput)
	return out, ok
}

func (s *scenarioState) priorCoverageOutput() (qg.CoverageOutput, bool) {
	out, ok := s.stepOpts["coverageOutput"].(qg.CoverageOutput)
	return out, ok
}

func (s *scenarioState) priorQualityScopeInventoryOutput() (qg.QualityScopeInventoryOutput, bool) {
	out, ok := s.stepOpts["qualityScopeInventoryOutput"].(qg.QualityScopeInventoryOutput)
	return out, ok
}

func (s *scenarioState) runCrap(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
) error {
	resolver := s.resolverForStep(stepCrap)
	gocycloVal, gOk := s.gocycloToolValue()
	if !gOk {
		gocycloVal = qg.GocycloToolValue{}
	}
	maxVal, ok := s.maxScore()
	if !ok || maxVal <= 0 {
		return qg.Crap(
			ctx,
			display,
			resolver,
			store,
			mem,
			root,
			qg.CoverageOutput{},
			qg.QualityScopeInventoryOutput{},
			qg.CrapThreshold{},
			gocycloVal,
			s.crapOptions()...,
		)
	}

	covOut, hasCov := s.priorCoverageOutput()
	if !hasCov {
		threshold := s.crapThreshold()
		return qg.Crap(
			ctx,
			display,
			resolver,
			store,
			mem,
			root,
			qg.CoverageOutput{},
			qg.QualityScopeInventoryOutput{},
			threshold,
			gocycloVal,
			s.crapOptions()...,
		)
	}
	threshold := s.crapThreshold()
	inv, _ := s.priorQualityScopeInventoryOutput()
	return qg.Crap(ctx, display, resolver, store, mem, root, covOut, inv, threshold, gocycloVal, s.crapOptions()...)
}

func (s *scenarioState) runDuration(
	ctx context.Context,
	display qg.CommandRunner,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
) error {
	threshold, ok := s.durationThreshold()
	if !ok {
		return qg.Duration(ctx, display, store, mem, root, qg.TestOutput{}, qg.DurationThreshold{})
	}
	testOut, ok := s.priorTestOutput()
	if !ok {
		return qg.Duration(ctx, display, store, mem, root, qg.TestOutput{}, threshold)
	}
	return qg.Duration(ctx, display, store, mem, root, testOut, threshold)
}

func (s *scenarioState) runMutationKills(
	ctx context.Context,
	display qg.CommandRunner,
	resolver qg.ToolResolver,
	store *qg.ArtifactStore,
	mem qg.FileOps,
	root string,
	scope qg.QualityScope,
) error {
	minRate, ok := s.minKillRate()
	var threshold qg.MinKillRateThreshold
	if ok {
		threshold = qg.MinKillRate(minRate)
	} else if err := qg.MutationKillRate(qg.MutationKillsOutput{}, threshold); err != nil {
		return err
	}

	opts := s.mutationOptions()

	gremlinsVal, gOk := s.gremlinsToolValueForStep(stepMutationKills)
	if !gOk {
		gremlinsVal = qg.GremlinsToolValue{}
	}

	mr, err := qg.NewMutationRunner(display, resolver, store, mem)
	if err != nil {
		return err
	}
	inv, _ := s.priorQualityScopeInventoryOutput()
	out, err := mr.Kill(ctx, root, scope, inv, gremlinsVal, opts...)
	if err != nil {
		return err
	}
	if err := qg.MutationKillRate(out, threshold); err != nil {
		return err
	}
	s.stepOpts["mutationKillsOutput"] = out
	return nil
}
