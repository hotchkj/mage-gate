// Vision: Internal step wiring from public API to harness—tokens, stores, and silent/verbose display error shaping.
package gate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

func crapValidateCoveragePrerequisites(store *ArtifactStore, covOutput *CoverageOutput) error {
	if err := validateCoverageOutputToken(covOutput); err != nil {
		return err
	}
	return requireUpstreamArtifact(store, covOutput.stepID, "coverage.out")
}

func crapValidateCore(
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	store *ArtifactStore,
	covOutput *CoverageOutput,
	maxScore CrapThreshold,
) error {
	if err := validateMaxScore(maxScore); err != nil {
		return err
	}
	if err := requireResolverStepDeps(runner, resolver, fileOps); err != nil {
		return err
	}
	if err := requireStoreDeps(runner, fileOps, store); err != nil {
		return err
	}
	return crapValidateCoveragePrerequisites(store, covOutput)
}

func crapValidatePrerequisites(
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	store *ArtifactStore,
	covOutput *CoverageOutput,
	maxScore CrapThreshold,
	gocycloSpec GocycloToolValue,
) error {
	if err := validateGocycloToolSpec(gocycloSpec); err != nil {
		return err
	}
	return crapValidateCore(runner, resolver, fileOps, store, covOutput, maxScore)
}

func requireStoreDeps(runner CommandRunner, fileOps FileOps, store *ArtifactStore) error {
	if runner == nil {
		return fmt.Errorf("%w: CommandRunner", ErrNilDependency)
	}
	if fileOps == nil {
		return fmt.Errorf("%w: FileOps", ErrNilDependency)
	}
	if store == nil {
		return fmt.Errorf("%w: Store", ErrNilDependency)
	}
	return nil
}

const (
	errMsgCoveredTestOutputEmptyStepID          = "CoveredTestOutput has empty stepID"
	errMsgCoveredTestOutputEmptyRunPackages     = "CoveredTestOutput has empty run-target packages"
	errMsgCoveredTestOutputEmptyProductionScope = "CoveredTestOutput has empty production scope"
)

func validateCoveredTestToken(coveredOutput *CoveredTestOutput) error {
	if coveredOutput == nil {
		return ErrCoveredTestRequired
	}
	if coveredOutput.stepID == "" {
		return fmt.Errorf("%w: %s", ErrMissingValue, errMsgCoveredTestOutputEmptyStepID)
	}
	if coveredOutput.packages.Packages() == "" {
		return fmt.Errorf("%w: %s", ErrMissingValue, errMsgCoveredTestOutputEmptyRunPackages)
	}
	if qualityScopePackages(coveredOutput.qualityScope) == "" {
		return fmt.Errorf("%w: %s", ErrMissingValue, errMsgCoveredTestOutputEmptyProductionScope)
	}
	return nil
}

func validateTestOutputToken(out TestOutput) error {
	if out.stepID == "" {
		return fmt.Errorf("%w: TestOutput stepID is empty", ErrMissingValue)
	}
	if _, err := validateGoTestPackagePattern(out.scope.Packages(), ErrMissingValue); err != nil {
		return fmt.Errorf("%w: TestOutput package scope: %w", ErrMissingValue, err)
	}
	return nil
}

func validateCoverageOutputToken(out *CoverageOutput) error {
	if out == nil || out.stepID == "" {
		return fmt.Errorf("%w: CoverageOutput stepID is empty", ErrMissingValue)
	}
	if _, err := validateGoTestPackagePattern(qualityScopePackages(out.qualityScope), ErrMissingValue); err != nil {
		return fmt.Errorf("%w: CoverageOutput quality scope packages: %w", ErrMissingValue, err)
	}
	return nil
}

func requireUpstreamArtifact(store *ArtifactStore, stepID, artifactName string) error {
	if stepID == "" {
		return fmt.Errorf("%w: upstream stepID is empty", ErrMissingValue)
	}
	if !store.Has(stepID, artifactName) {
		return fmt.Errorf("%w: artifact %s/%s not found", ErrMissingValue, stepID, artifactName)
	}
	return nil
}

func wrapStepError(name string, runner CommandRunner, err error) error {
	if err == nil {
		return nil
	}
	return wrapStepErrorWithMode(name, RunnerOutputMode(runner), err, runnerAsStepDisplay(runner))
}

func wrapStepErrorWithMode(name string, mode OutputMode, err error, display stepDisplay) error {
	if err == nil {
		return nil
	}
	emit := func(diagErr error) error {
		emitDiagnosticIfPossible(display, diagErr)
		return diagErr
	}
	switch stepOutputModeForDiagnostics(mode) {
	case OutputModeVerbose:
		return err
	case OutputModeAgent:
		return emit(stepDiagnostic(name, err))
	default:
		return emit(stepDiagnostic(name, err))
	}
}

func stepOutputModeForDiagnostics(mode OutputMode) OutputMode {
	switch mode {
	case OutputModeAgent, OutputModeVerbose:
		return mode
	default:
		return OutputModeVerbose
	}
}

func emitDiagnosticIfPossible(display stepDisplay, err error) {
	var de *DiagnosticError
	if !errors.As(err, &de) {
		return
	}
	if display == nil {
		return
	}
	display.EmitDiagnostic(err.Error())
}

// wrapHarnessCleanup applies the same display-mode shaping to harness temp-dir cleanup failures as step failures.
func wrapHarnessCleanup(stepName string, runner CommandRunner, cleanupErr error) error {
	if cleanupErr == nil {
		return nil
	}
	return wrapStepError(stepName, runner, fmt.Errorf("harness cleanup: %w", cleanupErr))
}

func validateLintCustomBinary(cfg lintConfig) error {
	if cfg.customLintSpec != "" && cfg.customGCLPath == "" {
		return newValidationError(
			"lint",
			"CustomLintToolSpec requires CustomGCL so the custom golangci-lint build runs from a module directory",
			ErrInvalidOption,
		)
	}
	if cfg.customGCLPath != "" && cfg.customLintSpec == "" {
		return newValidationError(
			"lint",
			"CustomGCL requires CustomLintToolSpec so the custom golangci-lint builder is explicitly pinned",
			ErrInvalidOption,
		)
	}
	if cfg.customLintSpec != "" {
		if err := cmdrunner.ValidateToolSpec(cfg.customLintSpec); err != nil {
			return newValidationError(
				"lint",
				fmt.Sprintf("CustomLintToolSpec must be in format 'package@version', got %q", cfg.customLintSpec),
				ErrInvalidOption,
			)
		}
	}
	return nil
}

func lintValidateBeforeHarness(
	packages PackageScope,
	configPath LintConfigValue,
	toolSpec LintToolValue,
	cfg lintConfig,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
) error {
	if _, err := validateGoTestPackagePattern(packages.Packages(), ErrPackageScopeEmpty); err != nil {
		return err
	}
	if err := validateLintInputs(configPath, toolSpec); err != nil {
		return err
	}
	if err := validateLintCustomBinary(cfg); err != nil {
		return err
	}
	return requireResolverStepDeps(runner, resolver, fileOps)
}

//nolint:gocritic // Opaque value token
func lintValidateToolchainBeforeHarness(
	packages PackageScope,
	lt LintToolchain,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
) error {
	if err := validateLintToolchain(lt); err != nil {
		return err
	}
	return lintValidateBeforeHarness(
		packages,
		LintConfig(lt.configPath),
		LintToolSpec(lt.lintToolSpec),
		lt.lintConfig(),
		runner,
		resolver,
		fileOps,
	)
}

func mutationValidateBeforeScan(
	qualityScope QualityScope,
	gremlinsSpec GremlinsToolValue,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	store *ArtifactStore,
) error {
	if err := validateQualityScope(qualityScope); err != nil {
		return err
	}
	if err := validateGremlinsToolSpec("mutationscan", gremlinsSpec); err != nil {
		return err
	}
	if err := requireStoreDeps(runner, fileOps, store); err != nil {
		return err
	}
	if resolver == nil {
		return fmt.Errorf("%w: ToolResolver", ErrNilDependency)
	}
	return nil
}

func mutationValidateBeforeHarnessForKills(
	qualityScope QualityScope,
	minKillRate MinKillRateThreshold,
	gremlinsSpec GremlinsToolValue,
	runner CommandRunner,
	resolver ToolResolver,
	fileOps FileOps,
	store *ArtifactStore,
) error {
	if err := validateQualityScope(qualityScope); err != nil {
		return err
	}
	if err := validateMinKillRate(minKillRate); err != nil {
		return err
	}
	if err := validateGremlinsToolSpec("mutationkills", gremlinsSpec); err != nil {
		return err
	}
	if err := requireStoreDeps(runner, fileOps, store); err != nil {
		return err
	}
	if resolver == nil {
		return fmt.Errorf("%w: ToolResolver", ErrNilDependency)
	}
	return nil
}

func gateRoot(root string) string {
	return root
}

func validateRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: root must not be empty", ErrMissingValue)
	}
	return nil
}

func validateRootAndStoreDeps(root string, runner CommandRunner, fileOps FileOps, store *ArtifactStore) error {
	if err := validateRoot(root); err != nil {
		return err
	}
	return requireStoreDeps(runner, fileOps, store)
}
