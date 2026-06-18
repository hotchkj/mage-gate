// Vision: Public step functions as thin adapters—validate options, build harness deps, delegate to internal runners.
package gate

import (
	"context"
	"errors"
	"fmt"

	"github.com/hotchkj/mage-gate/internal/harness"
)

func validateQualityScope(qualityScope QualityScope) error {
	_, err := validateGoTestPackagePattern(qualityScopePackages(qualityScope), ErrQualityScopeEmpty)
	return err
}

func requireResolverStepDeps(
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
) error {
	if runner == nil {
		return fmt.Errorf("%w: CommandRunner", ErrNilDependency)
	}
	if resolver == nil {
		return fmt.Errorf("%w: ToolResolver", ErrNilDependency)
	}
	if fileOps == nil {
		return fmt.Errorf("%w: FileOps", ErrNilDependency)
	}
	return nil
}

// beginTestHarness builds the store-backed harness for [Test] ("test"…) or [CoveredTest] ("coveredtest"…) prefixes.
func beginTestHarness(
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root, pkgPattern, stepIDPrefix string,
) (id string, harn *harness.StepRunner, err error) {
	if depErr := requireStoreDeps(runner, fileOps, store); depErr != nil {
		return "", nil, depErr
	}
	id = nextID(stepIDPrefix)
	harn, err = harness.NewStepRunner(gateRoot(root), "", pkgPattern, runner, fileOps, store, id)
	if err != nil {
		return "", nil, fmt.Errorf("create harness: %w", err)
	}
	return id, harn, nil
}

func newDiscardHarness(
	root string,
	packages PackageScope,
	runner CommandRunner,
	fileOps FileOps,
	opts ...harness.StepRunnerOption,
) (*harness.StepRunner, error) {
	return harness.NewStepRunner(
		gateRoot(root),
		"",
		packages.Packages(),
		runner,
		fileOps,
		harness.NewDiscardArtifactStore(),
		"",
		opts...,
	)
}

type lintHarnessStep func(context.Context, *harness.StepRunner, LintToolchain) error

// runLintLike runs shared Lint/Format orchestration: progress line, validation, harness, cleanup, error wrap.
//
//nolint:gocritic // Opaque value token
func runLintLike(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	root string,
	packages PackageScope,
	lt LintToolchain,
	stepLine, stepName string,
	run lintHarnessStep,
) (err error) {
	emitStepStart(runner, stepLine, "")
	if checkErr := lintValidateToolchainBeforeHarness(
		packages, lt, runner, resolver, fileOps,
	); checkErr != nil {
		return checkErr
	}
	harn, err := newDiscardHarness(
		root,
		packages,
		runner,
		fileOps,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup(stepName, runner, harn.Cleanup())) }()
	return wrapStepError(stepName, runner, run(ctx, harn, lt))
}

//nolint:gocritic // Opaque value token
func lintHarnessStepLint(ctx context.Context, h *harness.StepRunner, lt LintToolchain) error {
	return h.StepLint(
		ctx,
		lt.configPath,
		lt.customGCLPath,
		lt.customLintSpec,
		lt.lintToolSpec,
		lt.lintArgs,
	)
}

//nolint:gocritic // Opaque value token
func lintHarnessStepFormat(ctx context.Context, h *harness.StepRunner, lt LintToolchain) error {
	return h.StepFormat(
		ctx,
		lt.configPath,
		lt.customGCLPath,
		lt.customLintSpec,
		lt.lintToolSpec,
		lt.lintArgs,
	)
}

// Lint runs golangci-lint over [PackageScope].
// [QualityScope] applies to measurement steps ([CoveredTest], [Coverage], mutation scan/kill), not lint.
//
//nolint:gocritic // Opaque value token
func Lint(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	root string,
	packages PackageScope,
	lt LintToolchain,
) error {
	if err := validateRoot(root); err != nil {
		return err
	}
	return runLintLike(
		ctx, runner, resolver, fileOps, root, packages, lt,
		stepLineLint, "lint", lintHarnessStepLint,
	)
}

// Format runs golangci-lint fmt (apply) over [PackageScope].
//
//nolint:gocritic // Opaque value token
func Format(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	root string,
	packages PackageScope,
	lt LintToolchain,
) error {
	if err := validateRoot(root); err != nil {
		return err
	}
	return runLintLike(
		ctx, runner, resolver, fileOps, root, packages, lt,
		stepLineFormat, "format", lintHarnessStepFormat,
	)
}

// Compile is compile-only verification over [PackageScope] (not a release build; [QualityScope] filters do not apply).
// Extra flags go through [CompileArgs]; avoid artifact flags like -o here—use consumer-owned release targets instead.
//
//nolint:gocritic // Opaque value token
func Compile(
	ctx context.Context,
	runner CommandRunner,
	fileOps FileOps,
	root string,
	packages PackageScope,
	opts ...CompileOption,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineCompile, "")
	if _, err = validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); err != nil {
		return err
	}
	cfg := compileConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if runner == nil {
		return fmt.Errorf("%w: CommandRunner", ErrNilDependency)
	}
	if fileOps == nil {
		return fmt.Errorf("%w: FileOps", ErrNilDependency)
	}
	pkgs := packages.Packages()
	harn, err := harness.NewStepRunner(gateRoot(root), "", pkgs, runner, fileOps, harness.NewDiscardArtifactStore(), "")
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("compile", runner, harn.Cleanup())) }()
	return wrapStepError("compile", runner, harn.StepCompile(ctx, cfg.compileArgs))
}

// Vet runs go vet over [PackageScope] without [QualityScope] excludes (measurement-only concept).
//
//nolint:gocritic // Opaque value token
func Vet(
	ctx context.Context,
	runner CommandRunner,
	fileOps FileOps,
	root string,
	packages PackageScope,
	opts ...VetOption,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineVet, "")
	if _, err = validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); err != nil {
		return err
	}
	cfg := vetConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if runner == nil {
		return fmt.Errorf("%w: CommandRunner", ErrNilDependency)
	}
	if fileOps == nil {
		return fmt.Errorf("%w: FileOps", ErrNilDependency)
	}
	pkgs := packages.Packages()
	harn, err := harness.NewStepRunner(gateRoot(root), "", pkgs, runner, fileOps, harness.NewDiscardArtifactStore(), "")
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("vet", runner, harn.Cleanup())) }()
	return wrapStepError("vet", runner, harn.StepVet(ctx, cfg.vetArgs))
}

// Deadcode runs deadcode over [PackageScope] without [QualityScope] excludes (measurement-only concept).
//
//nolint:gocritic // Opaque value token
func Deadcode(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	root string,
	packages PackageScope,
	toolSpec DeadcodeToolValue,
	opts ...DeadcodeOption,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineDeadcode, "")
	if _, checkErr := validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); checkErr != nil {
		return checkErr
	}
	cfg := defaultDeadcodeConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := validateDeadcodeToolSpec(toolSpec); checkErr != nil {
		return checkErr
	}
	if checkErr := requireResolverStepDeps(runner, resolver, fileOps); checkErr != nil {
		return checkErr
	}
	harn, err := newDiscardHarness(
		root,
		packages,
		runner,
		fileOps,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("deadcode", runner, harn.Cleanup())) }()
	return wrapStepError("deadcode", runner, harn.StepDeadcode(ctx, toolSpec.spec, cfg.args))
}

// MarkdownLint runs gomarklint from repo root with consumer-supplied args only (no [PackageScope]).
// Pass config via [MarkdownLintArgs] (for example MarkdownLintArgs("--config", ".gomarklint.json")).
//
//nolint:gocritic // Opaque value token
func MarkdownLint(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	root string,
	toolSpec MarkdownLintToolValue,
	opts ...MarkdownLintOption,
) (err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return rootErr
	}
	emitStepStart(runner, stepLineMarkdownLint, "")
	cfg := defaultMarkdownlintConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := validateMarkdownLintToolSpec(toolSpec); checkErr != nil {
		return checkErr
	}
	if checkErr := requireResolverStepDeps(runner, resolver, fileOps); checkErr != nil {
		return checkErr
	}
	harn, err := newDiscardHarness(
		root,
		PackageScope{},
		runner,
		fileOps,
		harness.WithToolResolver(resolver),
	)
	if err != nil {
		return fmt.Errorf("create harness: %w", err)
	}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("markdownlint", runner, harn.Cleanup())) }()
	return wrapStepError("markdownlint", runner, harn.StepMarkdownLint(ctx, toolSpec.spec, cfg.args))
}

// Test runs go test without coverage flags. Returns a [TestOutput] for [Duration].
//
//nolint:gocritic // Opaque value token
func Test(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	packages PackageScope,
	opts ...TestOption,
) (out TestOutput, err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return TestOutput{}, rootErr
	}
	if _, checkErr := validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); checkErr != nil {
		return TestOutput{}, checkErr
	}
	cfg := defaultTestConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	qualifier := qualifierForTest(cfg)
	emitStepStart(runner, stepLineTest, qualifier)
	id, harn, err := beginTestHarness(runner, store, fileOps, root, packages.Packages(), "test")
	if err != nil {
		return TestOutput{}, err
	}
	out = TestOutput{stepID: id, scope: packages, qualifier: qualifier}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("test", runner, harn.Cleanup())) }()
	err = wrapStepError("test", runner, harn.StepTest(ctx, "", false, "", cfg.testArgs))
	if err != nil {
		return TestOutput{}, err
	}
	return out, nil
}

func validateCoveredTestInputs(
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	packages PackageScope,
	production QualityScope,
	inventory *QualityScopeInventoryOutput,
) error {
	if _, checkErr := validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); checkErr != nil {
		return checkErr
	}
	if checkErr := validateQualityScope(production); checkErr != nil {
		return checkErr
	}
	if checkErr := requireStoreDeps(runner, fileOps, store); checkErr != nil {
		return checkErr
	}
	return requireQualityScopeInventory(inventory, store, root, production)
}

func coveredTestConfig(opts []TestOption) (testConfig, error) {
	cfg := defaultTestConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if checkErr := rejectBuildTagArgs("coveredtest", cfg.testArgs); checkErr != nil {
		return testConfig{}, checkErr
	}
	return cfg, nil
}

// CoveredTest runs instrumented [go test]: [PackageScope] is run target; [QualityScope] seeds coverpkg/excludes.
// Returns [CoveredTestOutput] for [Coverage]/[Crap]; use [CoveredTestOutput.TestRun] for [Duration] input.
//
//nolint:gocritic // Opaque value token
func CoveredTest(
	ctx context.Context,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	packages PackageScope,
	production QualityScope,
	inventory QualityScopeInventoryOutput,
	opts ...TestOption,
) (out CoveredTestOutput, err error) {
	if rootErr := validateRoot(root); rootErr != nil {
		return CoveredTestOutput{}, rootErr
	}
	if checkErr := validateCoveredTestInputs(
		runner, store, fileOps, root, packages, production, &inventory,
	); checkErr != nil {
		return CoveredTestOutput{}, checkErr
	}
	cfg, checkErr := coveredTestConfig(opts)
	if checkErr != nil {
		return CoveredTestOutput{}, checkErr
	}
	qualifier := qualifierForCoveredTest(cfg, production)
	emitStepStart(runner, stepLineCoveredTest, qualifier)
	id, harn, err := beginTestHarness(runner, store, fileOps, root, packages.Packages(), "coveredtest")
	if err != nil {
		return CoveredTestOutput{}, err
	}
	out = CoveredTestOutput{stepID: id, packages: packages, qualityScope: production, qualifier: qualifier}
	defer func() { err = errors.Join(err, wrapHarnessCleanup("coveredtest", runner, harn.Cleanup())) }()
	commandScope := qualityScopeCommandScope(production, &inventory)
	err = wrapStepError(
		"coveredtest",
		runner,
		harn.StepCoveredTest(
			ctx,
			commandScope,
			false,
			cfg.testArgs,
		),
	)
	if err != nil {
		return CoveredTestOutput{}, err
	}
	return out, nil
}
