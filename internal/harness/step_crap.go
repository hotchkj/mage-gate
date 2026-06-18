// Vision: CRAP step: join coverage artifacts with gocyclo complexity, score via gatecheck, surface diagnostics.
package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

var (
	errCrapModuleMetadataEmpty     = errors.New("empty module metadata output")
	errCrapModuleMetadataExtraJSON = errors.New("unexpected extra JSON after module object")
	errCrapModuleMetadataPathEmpty = errors.New("module metadata Path is empty")
	errCrapModuleMetadataDirEmpty  = errors.New("module metadata Dir is empty")
)

// StepCrap loads coverage.out (and gocyclo output per spec) from the artifact store, then scores CRAP against crapMax.
func (h *StepRunner) StepCrap(
	ctx context.Context,
	crapMax float64,
	upstreamStepID string,
	commandScope *gatecheck.QualityScopeCommandScope,
	gocycloSpec string,
	crapExtraArgs []string,
) error {
	if upstreamStepID == "" {
		return fmt.Errorf("%w: upstream step ID not configured", ErrCrapFailed)
	}
	if h.toolResolver == nil {
		return fmt.Errorf("%w: ToolResolver is required", ErrCrapFailed)
	}
	if commandScope == nil {
		return fmt.Errorf("%w: quality scope command scope is nil", ErrCrapFailed)
	}
	if mkdirErr := h.fileOps.MkdirAll(h.artifacts.Dir(), artifactDirPerm); mkdirErr != nil {
		return fmt.Errorf("%w: create artifact dir: %w", ErrCrapFailed, mkdirErr)
	}

	coverageCommandPath, err := h.prepareCoverageForCrap(upstreamStepID, commandScope)
	if err != nil {
		return err
	}

	gocycloLogicalPath, coverFuncLogicalPath, modulePath, moduleDir, err := h.generateCrapReports(
		ctx, coverageCommandPath, commandScope, gocycloSpec, crapExtraArgs,
	)
	if err != nil {
		return err
	}

	_, testFilePatterns := commandScope.ThresholdPathFilters()
	return h.validateCrapOutput(
		gocycloLogicalPath, coverFuncLogicalPath, modulePath, moduleDir,
		crapMax, testFilePatterns,
	)
}

func (h *StepRunner) prepareCoverageForCrap(
	upstreamStepID string,
	commandScope *gatecheck.QualityScopeCommandScope,
) (string, error) {
	coverageData, readErr := h.store.Read(upstreamStepID, "coverage.out")
	if readErr != nil {
		return "", fmt.Errorf("%w: read coverage from store: %w", ErrCrapFailed, readErr)
	}
	rawFileOps, err := h.artifacts.FileOpsPath("coverage.out")
	if err != nil {
		return "", fmt.Errorf("%w: coverage artifact path: %w", ErrCrapFailed, err)
	}
	rawCommand, err := h.artifacts.CommandPath("coverage.out")
	if err != nil {
		return "", fmt.Errorf("%w: coverage argv path: %w", ErrCrapFailed, err)
	}
	if writeErr := h.fileOps.WriteFile(rawFileOps, coverageData, artifactFilePerm); writeErr != nil {
		return "", fmt.Errorf("%w: write coverage file: %w", ErrCrapFailed, writeErr)
	}
	return h.filteredCoverageCommandOrOriginal(coverageData, rawCommand, commandScope.CoverageProfileFilter())
}

func (h *StepRunner) filteredCoverageCommandOrOriginal(
	coverageData []byte,
	rawCoverageCommand string,
	filter gatecheck.CoverageProfileFilter,
) (string, error) {
	if !filter.Needed() {
		return rawCoverageCommand, nil
	}
	filteredData, filterErr := filter.Apply(string(coverageData))
	if filterErr != nil {
		return "", fmt.Errorf("%w: filter coverage profile: %w", ErrCrapFailed, filterErr)
	}
	if !coverageProfileHasDataLines(filteredData) {
		return "", fmt.Errorf("%w: %w", ErrCrapFailed, gatecheck.ErrAllPackagesExcluded)
	}
	filteredFileOps, err := h.artifacts.FileOpsPath("coverage-filtered.out")
	if err != nil {
		return "", fmt.Errorf("%w: filtered coverage artifact path: %w", ErrCrapFailed, err)
	}
	filteredCommand, err := h.artifacts.CommandPath("coverage-filtered.out")
	if err != nil {
		return "", fmt.Errorf("%w: filtered coverage argv path: %w", ErrCrapFailed, err)
	}
	if writeErr := h.fileOps.WriteFile(filteredFileOps, []byte(filteredData), artifactFilePerm); writeErr != nil {
		return "", fmt.Errorf("%w: write filtered coverage: %w", ErrCrapFailed, writeErr)
	}
	return filteredCommand, nil
}

func (h *StepRunner) generateCrapReports(
	ctx context.Context,
	coverageCommandPath string,
	commandScope *gatecheck.QualityScopeCommandScope,
	gocycloSpec string,
	crapExtraArgs []string,
) (gocycloLogicalPath, coverFuncLogicalPath, modulePath, moduleDir string, err error) {
	dirs, err := h.gocycloHostDirsFromCommandScope(commandScope)
	if err != nil {
		return "", "", "", "", err
	}

	gocycloLogicalPath, err = h.artifacts.FileOpsPath("gocyclo.txt")
	if err != nil {
		return "", "", "", "", fmt.Errorf("%w: gocyclo report path: %w", ErrCrapFailed, err)
	}
	if writeErr := h.writeGocycloReport(ctx, dirs, gocycloLogicalPath, gocycloSpec, crapExtraArgs); writeErr != nil {
		return "", "", "", "", writeErr
	}

	coverFuncLogicalPath, err = h.artifacts.FileOpsPath("cover-func.txt")
	if err != nil {
		return "", "", "", "", fmt.Errorf("%w: cover-func report path: %w", ErrCrapFailed, err)
	}
	if writeErr := h.writeCoverFuncReport(ctx, coverageCommandPath, coverFuncLogicalPath); writeErr != nil {
		return "", "", "", "", writeErr
	}

	modulePath, moduleDir, err = h.fetchModuleCoordinates(ctx)
	if err != nil {
		return "", "", "", "", err
	}

	return gocycloLogicalPath, coverFuncLogicalPath, modulePath, moduleDir, nil
}

func (h *StepRunner) gocycloHostDirsFromCommandScope(
	commandScope *gatecheck.QualityScopeCommandScope,
) ([]string, error) {
	rootRelDirs, err := commandScope.GocycloPkgDirsRootRel()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCrapFailed, err)
	}
	dirs := make([]string, 0, len(rootRelDirs))
	for _, dirRootRel := range rootRelDirs {
		dir, resolveErr := h.resolveCrapPackageDir(dirRootRel)
		if resolveErr != nil {
			return nil, resolveErr
		}
		dirs = append(dirs, fsnorm.Canonical(dir))
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("%w: %w", ErrCrapFailed, gatecheck.ErrAllPackagesExcluded)
	}
	return dirs, nil
}

func (h *StepRunner) validateCrapOutput(
	gocycloLogicalPath, coverFuncLogicalPath, modulePath, moduleDir string,
	crapMax float64,
	testFilePatterns []string,
) error {
	gocycloData, err := h.fileOps.ReadFile(gocycloLogicalPath) // #nosec G304 -- logical artifact path from h.artifacts
	if err != nil {
		return fmt.Errorf("%w: read gocyclo report: %w", ErrCrapFailed, err)
	}
	coverFuncData, err := h.fileOps.ReadFile(coverFuncLogicalPath) // #nosec G304 -- logical artifact path from h.artifacts
	if err != nil {
		return fmt.Errorf("%w: read cover-func report: %w", ErrCrapFailed, err)
	}

	result, err := gatecheck.Crap(
		string(gocycloData),
		string(coverFuncData),
		modulePath,
		moduleDir,
		crapMax,
		testFilePatterns,
	)
	if err != nil {
		return fmt.Errorf("%w: crap check: %w", ErrCrapFailed, err)
	}
	if !result.Passed {
		return fmt.Errorf("%w: %s", ErrCrapFailed, gatecheck.FormatCrapReport(result))
	}
	return nil
}

func (h *StepRunner) resolveCrapPackageDir(pkgDirRootRel string) (string, error) {
	dirInput := pkgDirRootRel
	if strings.TrimSpace(dirInput) == "" {
		dirInput = "."
	}
	logicalRel, err := fileopspath.LogicalContainedRelative(h.root, dirInput)
	if err != nil {
		return "", fmt.Errorf("%w: gocyclo package dir: %w", ErrCrapFailed, err)
	}
	if logicalRel == "." || logicalRel == "" {
		return h.root, nil
	}
	return filepath.Join(h.root, filepath.FromSlash(logicalRel)), nil
}

func (h *StepRunner) writeCoverFuncReport(ctx context.Context, coverageCommandPath, coverFuncLogicalPath string) error {
	result, err := h.runCommand(ctx, h.root, "go", "tool", "cover", "-func="+coverageCommandPath)
	if err != nil {
		return fmt.Errorf("%w: cover-func report: %w", ErrCrapFailed, err)
	}
	if err := h.fileOps.WriteFile(coverFuncLogicalPath, []byte(result.Stdout), artifactFilePerm); err != nil {
		return fmt.Errorf("%w: write cover-func report: %w", ErrCrapFailed, err)
	}
	return nil
}

func (h *StepRunner) fetchModuleCoordinates(ctx context.Context) (modulePath, moduleDir string, err error) {
	result, err := h.runCommand(ctx, h.root, "go", "list", "-m", "-json")
	if err != nil {
		return "", "", fmt.Errorf("%w: module metadata: %w", ErrCrapFailed, err)
	}
	modulePath, moduleDir, err = parseCrapModuleCoordinatesJSON(result.Stdout)
	if err != nil {
		return "", "", fmt.Errorf("%w: module metadata: %w", ErrCrapFailed, err)
	}
	return modulePath, moduleDir, nil
}

func parseCrapModuleCoordinatesJSON(raw string) (modulePath, moduleDir string, err error) {
	data := strings.TrimSpace(raw)
	if data == "" {
		return "", "", errCrapModuleMetadataEmpty
	}
	dec := json.NewDecoder(strings.NewReader(data))
	var module struct {
		Path string
		Dir  string
	}
	if err := dec.Decode(&module); err != nil {
		return "", "", err
	}
	if trailing := strings.TrimSpace(data[int(dec.InputOffset()):]); trailing != "" {
		return "", "", errCrapModuleMetadataExtraJSON
	}
	modulePath = strings.TrimSpace(module.Path)
	if modulePath == "" {
		return "", "", errCrapModuleMetadataPathEmpty
	}
	moduleDir = strings.TrimSpace(module.Dir)
	if moduleDir == "" {
		return "", "", errCrapModuleMetadataDirEmpty
	}
	return modulePath, moduleDir, nil
}

func (h *StepRunner) writeGocycloReport(
	ctx context.Context, dirs []string, logicalPath, gocycloSpec string, crapExtraArgs []string,
) error {
	base := append([]string{"-over", "0"}, crapExtraArgs...)
	binary, baseArgs, err := h.toolResolver.ResolveToolCommand(ctx, "gocyclo", gocycloSpec, base)
	if err != nil {
		return fmt.Errorf("%w: resolve gocyclo command: %w", ErrCrapFailed, err)
	}
	args := append(append([]string{}, baseArgs...), dirs...)
	result, runErr := h.runCommand(ctx, h.root, binary, args...)
	if runErr != nil {
		return fmt.Errorf("%w: gocyclo report: %w", ErrCrapFailed, runErr)
	}
	if err := h.fileOps.WriteFile(logicalPath, []byte(result.Stdout), artifactFilePerm); err != nil {
		return fmt.Errorf("%w: write gocyclo report: %w", ErrCrapFailed, err)
	}
	return nil
}
