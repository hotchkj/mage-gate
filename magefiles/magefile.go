//go:build mage
// +build mage

// Quality gate targets for this repository.
// Vision: Zero-flag mage gate from repo root—ordered steps, CI-aware output mode, always-on progress lines.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	qg "github.com/hotchkj/mage-gate/gate"
)

// errIntegrationNoPackages is returned when mage integrationtests runs but
// [integrationtests].packages is empty.
var errIntegrationNoPackages = errors.New(
	`integrationtests: gate.toml [integrationtests].packages is empty; set packages or use "mage unittests"`,
)

var errRepositoryRootNotFound = errors.New(
	`resolve repository root: cannot find repo root go.mod`,
)
var errRepositoryRootGetwd = errors.New("resolve repository root: getwd failed")

const policyPath = "gate.toml"

func resolveRepositoryRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Join(errRepositoryRootGetwd, err)
	}

	if _, statErr := os.Stat(filepath.Join(cwd, "go.mod")); statErr == nil {
		return ".", nil
	}
	if _, statErr := os.Stat(filepath.Join(cwd, "..", "go.mod")); statErr == nil {
		return "..", nil
	}
	return "", fmt.Errorf("%w", errRepositoryRootNotFound)
}

func withRepositoryRoot[T any](fn func(string) (T, error)) (T, error) {
	repoRoot, err := resolveRepositoryRoot()
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(repoRoot)
}

func withRepositoryRootNoResult(fn func(string) error) error {
	_, err := withRepositoryRoot(func(repoRoot string) (struct{}, error) {
		return struct{}{}, fn(repoRoot)
	})
	return err
}

func loadConfigAndScope() (config, qg.QualityScope, qg.PackageScope, error) {
	repoRoot, err := resolveRepositoryRoot()
	if err != nil {
		return config{}, qg.QualityScope{}, qg.PackageScope{}, err
	}
	cfg, err := loadConfig(filepath.Join(repoRoot, policyPath), readPolicyFile)
	if err != nil {
		return config{}, qg.QualityScope{}, qg.PackageScope{}, fmt.Errorf("load config: %w", err)
	}
	qualityScope, err := qg.NewQualityScope(cfg.packages(), qualityScopeOptions(&cfg)...)
	if err != nil {
		return config{}, qg.QualityScope{}, qg.PackageScope{}, fmt.Errorf("create quality scope: %w", err)
	}
	pkgScope, err := qg.NewPackageScope(cfg.packages())
	if err != nil {
		return config{}, qg.QualityScope{}, qg.PackageScope{}, fmt.Errorf("create package scope: %w", err)
	}
	return cfg, qualityScope, pkgScope, nil
}

func runGateSteps(
	ctx context.Context,
	runner qg.CommandRunner,
	resolver qg.ToolResolver,
	fileOps qg.FileOps,
	root string,
	pkgScope qg.PackageScope,
	cfg *config,
) error {
	steps := []struct {
		fn func() error
	}{
		{func() error {
			lt, ltErr := lintToolchain(cfg)
			if ltErr != nil {
				return ltErr
			}
			return qg.Lint(ctx, runner, resolver, fileOps, root, pkgScope, lt)
		}},
		{func() error {
			return qg.Deadcode(
				ctx, runner, resolver, fileOps, root, pkgScope,
				deadcodeToolSpec(cfg), deadcodeOptions(cfg)...,
			)
		}},
		{func() error { return qg.Vet(ctx, runner, fileOps, root, pkgScope) }},
		{func() error { return qg.Compile(ctx, runner, fileOps, root, pkgScope) }},
	}
	for _, st := range steps {
		if fnErr := st.fn(); fnErr != nil {
			return fnErr
		}
	}
	if markdownlintEnabled(cfg) {
		return qg.MarkdownLint(
			ctx, runner, resolver, fileOps, root,
			markdownlintToolSpec(cfg), markdownlintOpts(cfg)...,
		)
	}
	return nil
}

func runGatePostTest(
	ctx context.Context,
	runner qg.CommandRunner,
	resolver qg.ToolResolver,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	root string,
	cfg *config,
	unitCov *qg.CoveredTestOutput,
	inventory *qg.QualityScopeInventoryOutput,
) error {
	covOut, covErr := qg.Coverage(ctx, runner, store, fileOps, root, *unitCov, coverageMin(cfg))
	if covErr != nil {
		return covErr
	}
	if crapErr := qg.Crap(
		ctx, runner, resolver, store, fileOps, root, covOut, *inventory,
		crapMax(cfg), gocycloToolSpec(cfg), crapOptions(cfg)...,
	); crapErr != nil {
		return crapErr
	}
	testRun, trErr := unitCov.TestRun()
	if trErr != nil {
		return trErr
	}
	return qg.Duration(
		ctx, runner, store, fileOps, root, testRun, durationMax(cfg),
	)
}

// runIntegrationPass runs gate.toml [integrationtests] without coverage flags so it does not
// merge into the coverage profile from the primary pass. When requirePackages is true and
// packages is empty, returns errIntegrationNoPackages. When requirePackages is false,
// an empty packages string skips the phase (no integration tests configured).
func runIntegrationPass(
	ctx context.Context,
	runner qg.CommandRunner,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	root string,
	cfg *config,
	requirePackages bool,
) error {
	pkgs := strings.TrimSpace(cfg.Integrationtests.Packages)
	if pkgs == "" {
		if requirePackages {
			return errIntegrationNoPackages
		}
		return nil
	}
	integrationScope, err := qg.NewQualityScope(pkgs, qualityScopeOptions(cfg)...)
	if err != nil {
		return fmt.Errorf("integration quality scope: %w", err)
	}
	integrationPkgs, err := qg.NewPackageScope(integrationScope.Packages())
	if err != nil {
		return fmt.Errorf("integration package scope: %w", err)
	}
	_, err = qg.Test(
		ctx, runner, store, fileOps, root, integrationPkgs,
		integrationPassOpts(cfg)...,
	)
	if err != nil {
		return fmt.Errorf("integration pass: %w", err)
	}
	return nil
}

// newGateMutationRunner constructs the [qg.MutationRunner] used in [runGateMutationPhase]. Tests
// replace it to assert a single dry-run [qg.MutationRunner.Scan] per gate run.
var newGateMutationRunner = qg.NewMutationRunner

// gateMutationSitesCheck and gateMutationCoverageCheck are the composed consumers after Scan; tests
// may wrap them to count invocations or correlate tokens.
var (
	gateMutationSitesCheck    = qg.MutationSites
	gateMutationCoverageCheck = qg.MutationCoverage
)

func runGateMutationPhase(
	ctx context.Context,
	runner qg.CommandRunner,
	resolver qg.ToolResolver,
	store *qg.ArtifactStore,
	fileOps qg.FileOps,
	root string,
	scope qg.QualityScope,
	inventory *qg.QualityScopeInventoryOutput,
	cfg *config,
) error {
	mutOpts := gremlinsMutationOptions(cfg)
	mr, err := newGateMutationRunner(runner, resolver, store, fileOps)
	if err != nil {
		return err
	}
	scanOut, mutErr := mr.Scan(ctx, root, scope, *inventory, gremlinsToolSpec(cfg), mutOpts...)
	if mutErr != nil {
		return mutErr
	}
	if mutErr := gateMutationSitesCheck(scanOut, mutationSitesMax(cfg)); mutErr != nil {
		return mutErr
	}
	if !mutationCoverageCheckEnabled(cfg) {
		return nil
	}
	return gateMutationCoverageCheck(scanOut, mutationCoverageThreshold(cfg))
}

type gateRuntime struct {
	runner   qg.CommandRunner
	resolver qg.ToolResolver
	store    *qg.ArtifactStore
	fileOps  qg.FileOps
	root     string
	ctx      context.Context
}

func newGateRuntime() (gateRuntime, error) {
	repoRoot, rootErr := resolveRepositoryRoot()
	if rootErr != nil {
		return gateRuntime{}, rootErr
	}
	runner, runnerErr := newRunner()
	if runnerErr != nil {
		return gateRuntime{}, runnerErr
	}
	return gateRuntime{
		runner:   runner,
		resolver: newResolver(),
		store:    newArtifactStore(),
		fileOps:  newFileOps(),
		root:     repoRoot,
		ctx:      context.Background(),
	}, nil
}

func runGatePreTest(rt *gateRuntime, cfg *config, pkgScope qg.PackageScope) error {
	if stepsErr := runGateSteps(rt.ctx, rt.runner, rt.resolver, rt.fileOps, rt.root, pkgScope, cfg); stepsErr != nil {
		return stepsErr
	}
	return cfg.validateIntegrationConfig()
}

func runGateLoaded(
	rt *gateRuntime,
	cfg *config,
	scope qg.QualityScope,
	pkgScope qg.PackageScope,
) error {
	root := rt.root
	if preErr := runGatePreTest(rt, cfg, pkgScope); preErr != nil {
		return preErr
	}

	inventory, invErr := runQualityScopeInventoryPhase(rt.ctx, rt.runner, rt.store, rt.fileOps, root, scope)
	if invErr != nil {
		return invErr
	}
	unitCov, testErr := runCoverageTestPhase(
		rt.ctx, rt.runner, rt.store, rt.fileOps, cfg, pkgScope, scope, &inventory, root,
	)
	if testErr != nil {
		return testErr
	}
	if mutErr := runGateMutationPhase(
		rt.ctx, rt.runner, rt.resolver, rt.store, rt.fileOps, rt.root, scope, &inventory, cfg,
	); mutErr != nil {
		return mutErr
	}
	if postErr := runGatePostTest(
		rt.ctx, rt.runner, rt.resolver, rt.store, rt.fileOps, rt.root, cfg, &unitCov, &inventory,
	); postErr != nil {
		return postErr
	}
	// Integration `go test` is gate/Test only — not chained from `crap`, `coverage`, or `duration`.
	return runIntegrationPass(rt.ctx, rt.runner, rt.store, rt.fileOps, rt.root, cfg, false)
}

func runGate() error {
	cfg, scope, pkgScope, loadErr := loadConfigAndScope()
	if loadErr != nil {
		return loadErr
	}
	cfgPtr := &cfg
	rt, runtimeErr := newGateRuntime()
	if runtimeErr != nil {
		return runtimeErr
	}
	printProgress("Running gate:")
	if gateErr := runGateLoaded(&rt, cfgPtr, scope, pkgScope); gateErr != nil {
		return gateErr
	}
	printProgress("Succeeded")
	return nil
}

// Default target when mage is invoked with no arguments.
var Default = Gate

// Gate runs the full quality gate: lint, deadcode, vet, compile, optional markdownlint (when
// [markdownlint].tool_spec is set), unittests, mutationsites (gremlins
// site budget via --dry-run), optional mutationcoverage when [thresholds].mutation_coverage_min is set
// above 0, coverage, CRAP, duration, integrationtests (if [integrationtests].packages is set).
// Output mode is selected automatically from the CI environment variable.
func Gate() error {
	return runGate()
}

// Runtime entrypoints for lint/deadcode/compile/vet/test/coverage/duration/crap
// are defined in magefile_runtime_steps.go.
