// Vision: Dry-run mutation scan: gremlins JSON discovery, optional in-harness per-file cap, artifacts.
package harness

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

const mutationReportJSONLogicalName = "mutations.json"

func (sr *StepRunner) mutationRunCommandScopeInputs(
	commandScope *gatecheck.QualityScopeCommandScope,
	mutationExtraArgs []string,
	stepErr error,
) (effectiveTags, coverpkg string, excludeFileArgs, extraArgs []string, err error) {
	if commandScope == nil {
		return "", "", nil, nil, fmt.Errorf("%w: quality scope command scope is nil", stepErr)
	}
	effectiveTags, extraArgs = mergeBuildTags(commandScope.TagsCSV(), mutationExtraArgs)
	coverpkg, excludeFileArgs, err = sr.mutationGremlinsInputsFromCommandScope(commandScope, stepErr)
	if err != nil {
		return "", "", nil, nil, err
	}
	return effectiveTags, coverpkg, excludeFileArgs, extraArgs, nil
}

// StepMutationScan runs gremlins in --dry-run, validates mutations.json, and enforces per-file site
// counts in-harness when mutationSitesMax is not MaxInt; the public gate runner uses MaxInt so the
// site-budget is enforced by the separate MutationSites consumer check instead.
func (sr *StepRunner) StepMutationScan(
	ctx context.Context,
	mutationSitesMax int,
	commandScope *gatecheck.QualityScopeCommandScope,
	gremlinsSpec string,
	mutationExtraArgs []string,
) error {
	if err := sr.ensureMutationArtifactDir(); err != nil {
		return err
	}
	effectiveTags, coverpkg, excludeFileArgs, mutationExtraArgs, err := sr.mutationRunCommandScopeInputs(
		commandScope, mutationExtraArgs, ErrMutationSitesFailed,
	)
	if err != nil {
		return err
	}

	mutationsFileOps, mutationsCmd, err := sr.mutationArtifactPaths(ErrMutationSitesFailed)
	if err != nil {
		return err
	}
	tail := append([]string{"--dry-run"}, mutationExtraArgs...)
	if err := sr.runGremlinsUnleash(
		ctx,
		coverpkg,
		mutationsCmd,
		gremlinsSpec,
		effectiveTags,
		excludeFileArgs,
		tail,
		ErrMutationSitesFailed,
	); err != nil {
		return err
	}

	if err := sr.validateMutationOutput(mutationsFileOps, mutationSitesMax); err != nil {
		return err
	}

	if sr.stepID == "" {
		return nil
	}
	return sr.storeMutationArtifact(mutationsFileOps)
}

func (sr *StepRunner) mutationArtifactPaths(stepErr error) (fileOpsPath, commandPath string, err error) {
	fileOpsPath, err = sr.artifacts.FileOpsPath(mutationReportJSONLogicalName)
	if err != nil {
		return "", "", fmt.Errorf("%w: mutation artifact path: %w", stepErr, err)
	}
	commandPath, err = sr.artifacts.CommandPath(mutationReportJSONLogicalName)
	if err != nil {
		return "", "", fmt.Errorf("%w: mutation command path: %w", stepErr, err)
	}
	return fileOpsPath, commandPath, nil
}

func (sr *StepRunner) storeMutationArtifact(mutationsFileOpsPath string) error {
	return sr.storeMutationArtifactWithStepErr(
		mutationsFileOpsPath,
		ErrMutationSitesFailed,
		"gremlins run unleash",
	)
}

func (sr *StepRunner) storeMutationArtifactWithStepErr(
	mutationsFileOpsPath string,
	stepErr error,
	provTool string,
) error {
	if sr.stepID == "" {
		return fmt.Errorf("%w: step ID required to store mutation artifact", stepErr)
	}
	// #nosec G304 -- logical artifact path from sr.artifacts.FileOpsPath
	data, err := sr.fileOps.ReadFile(mutationsFileOpsPath)
	if err != nil {
		return fmt.Errorf("%w: read mutations for store: %w", stepErr, err)
	}
	prov := Provenance{StepID: sr.stepID, Tool: provTool, Packages: sr.packages}
	if storeErr := sr.store.Write(sr.stepID, mutationReportJSONLogicalName, data, prov); storeErr != nil {
		return fmt.Errorf("%w: store mutations: %w", stepErr, storeErr)
	}
	return nil
}

func (sr *StepRunner) ensureMutationArtifactDir() error {
	return sr.ensureMutationArtifactDirWithStepErr(ErrMutationSitesFailed)
}

func (sr *StepRunner) ensureMutationArtifactDirWithStepErr(stepErr error) error {
	// Containment was validated during NewStepRunner (newArtifactPaths). Use canonical logical
	// artifact directory for FileOps — never ResolveWithinRoot/HostPath for this layout.
	if err := sr.fileOps.MkdirAll(sr.artifacts.Dir(), artifactDirPerm); err != nil {
		return fmt.Errorf("%w: create artifact dir: %w", stepErr, err)
	}
	return nil
}

// runGremlinsUnleash runs gremlins unleash; failures wrap stepErr (mutation-site vs kill-rate sentinel).
// excludeFileArgs are full "--exclude-files=^…" gremlins flags; tailArgs are --dry-run (scan) and optional
// passthrough mutation args, in that order after excludes.
func (sr *StepRunner) runGremlinsUnleash(
	ctx context.Context,
	coverpkg string,
	mutationsCommandPath string,
	gremlinsSpec string,
	buildTags string,
	excludeFileArgs, tailArgs []string,
	stepErr error,
) error {
	if sr.toolResolver == nil {
		return fmt.Errorf("%w: ToolResolver is required", stepErr)
	}
	toolArgs := []string{
		"unleash",
		"-o",
		mutationsCommandPath,
		"--coverpkg=" + coverpkg,
	}
	if buildTags != "" {
		toolArgs = append(toolArgs, "--tags="+buildTags)
	}
	toolArgs = append(toolArgs, excludeFileArgs...)
	toolArgs = append(toolArgs, tailArgs...)
	binary, args, err := sr.toolResolver.ResolveToolCommand(ctx, "gremlins", gremlinsSpec, toolArgs)
	if err != nil {
		return fmt.Errorf("%w: resolve gremlins command: %w", stepErr, err)
	}
	if _, runErr := sr.runCommand(ctx, sr.root, binary, args...); runErr != nil {
		return fmt.Errorf("%w: gremlins run: %w", stepErr, runErr)
	}
	return nil
}

func formatMutationParseFailure(stepErr error, label string, checkErr error, mutationsData []byte) error {
	return fmt.Errorf(
		"%w: %s: %w\nraw gremlins mutations.json:\n%s",
		stepErr,
		label,
		checkErr,
		mutationsData,
	)
}

func (sr *StepRunner) validateMutationOutput(mutationsFileOpsPath string, mutationSitesMax int) error {
	// #nosec G304 -- logical artifact path from sr.artifacts.FileOpsPath
	mutationsData, readErr := sr.fileOps.ReadFile(mutationsFileOpsPath)
	if readErr != nil {
		return fmt.Errorf("%w: read mutations file: %w", ErrMutationSitesFailed, readErr)
	}

	result, checkErr := gatecheck.MutationSites(mutationsData, mutationSitesMax)
	if checkErr != nil {
		return formatMutationParseFailure(
			ErrMutationSitesFailed,
			"mutation-site check",
			checkErr,
			mutationsData,
		)
	}
	if !result.Passed {
		return fmt.Errorf("%w: %s", ErrMutationSitesFailed, gatecheck.FormatMutationReport(result))
	}
	return nil
}

func (sr *StepRunner) mutationGremlinsInputsFromCommandScope(
	commandScope *gatecheck.QualityScopeCommandScope,
	stepErr error,
) (coverpkg string, excludeFileArgs []string, err error) {
	sourceFiles, sourceErr := sr.rootRelativeGoSourceFiles(stepErr)
	if sourceErr != nil {
		return "", nil, sourceErr
	}
	scoped := commandScope.WithSourceInventory(sourceFiles)
	coverpkg, coverErr := scoped.CoverpkgCSV()
	if coverErr != nil {
		return "", nil, fmt.Errorf("%w: %w", stepErr, coverErr)
	}
	excludeFileArgs, exErr := scoped.MutationExcludeFileArgv()
	if exErr != nil {
		return "", nil, fmt.Errorf("%w: %w", stepErr, exErr)
	}
	return coverpkg, excludeFileArgs, nil
}

func (sr *StepRunner) rootRelativeGoSourceFiles(stepErr error) ([]string, error) {
	var out []string
	err := sr.fileOps.Walk(".", func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, ok, relErr := sr.rootRelativeGoSourceFile(path, info)
		if relErr != nil {
			return relErr
		}
		if !ok {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: walk mutation sources: %w", stepErr, err)
	}
	sort.Strings(out)
	return out, nil
}

func (sr *StepRunner) rootRelativeGoSourceFile(path string, info fs.FileInfo) (rel string, include bool, err error) {
	if info == nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
		return "", false, nil
	}
	root := fsnorm.Canonical(sr.root)
	path = fsnorm.Canonical(path)
	rel, err = fsnorm.Rel(root, path)
	if err != nil {
		return "", false, err
	}
	return rel, true, nil
}

// StepMutationKills runs gremlins in full mode (killing mutations), validates that mutations.json
// parses, and stores the artifact. The minKillRate argument is ignored; gate callers enforce kill-rate
// thresholds via MutationKillRate on the kills output token after retrieval (pass 0).
func (sr *StepRunner) StepMutationKills(
	ctx context.Context,
	minKillRate int,
	commandScope *gatecheck.QualityScopeCommandScope,
	gremlinsSpec string,
	mutationExtraArgs []string,
) error {
	if err := sr.ensureMutationArtifactDirWithStepErr(ErrMutationKillsFailed); err != nil {
		return err
	}
	effectiveTags, coverpkg, excludeFileArgs, mutationExtraArgs, err := sr.mutationRunCommandScopeInputs(
		commandScope, mutationExtraArgs, ErrMutationKillsFailed,
	)
	if err != nil {
		return err
	}

	mutationsFileOps, mutationsCmd, err := sr.mutationArtifactPaths(ErrMutationKillsFailed)
	if err != nil {
		return err
	}
	err = sr.runGremlinsUnleash(
		ctx,
		coverpkg,
		mutationsCmd,
		gremlinsSpec,
		effectiveTags,
		excludeFileArgs,
		append([]string(nil), mutationExtraArgs...),
		ErrMutationKillsFailed,
	)
	if err != nil {
		return err
	}

	if err := sr.validateMutationKillsArtifact(mutationsFileOps); err != nil {
		return err
	}

	if sr.stepID == "" {
		return nil
	}
	return sr.storeMutationArtifactForKills(mutationsFileOps)
}

func (sr *StepRunner) storeMutationArtifactForKills(mutationsFileOpsPath string) error {
	return sr.storeMutationArtifactWithStepErr(
		mutationsFileOpsPath,
		ErrMutationKillsFailed,
		"gremlins run unleash (full)",
	)
}

func (sr *StepRunner) validateMutationKillsArtifact(mutationsFileOpsPath string) error {
	// #nosec G304 -- logical artifact path from sr.artifacts.FileOpsPath
	mutationsData, readErr := sr.fileOps.ReadFile(mutationsFileOpsPath)
	if readErr != nil {
		return fmt.Errorf("%w: read mutations file: %w", ErrMutationKillsFailed, readErr)
	}

	_, checkErr := gatecheck.MutationKills(mutationsData, 0)
	if checkErr != nil {
		return formatMutationParseFailure(
			ErrMutationKillsFailed,
			"mutation-kills check",
			checkErr,
			mutationsData,
		)
	}
	return nil
}
