// Vision: Coverage step: merge profiles, apply coverpkg filters, run gatecheck thresholds, persist outputs for CRAP.
package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// coverageArtifactPaths holds paired FileOps and argv projections for coverage profiles so tool commands
// and store/FileOps reads stay aligned when CommandPath diverges from FileOpsPath later.
type coverageArtifactPaths struct {
	rawFileOps      string
	rawCommand      string
	filteredFileOps string
	filteredCommand string
}

// StepCoverage evaluates coverage against coverageMin using the upstream test step's stored profile when set.
func (h *StepRunner) StepCoverage(
	ctx context.Context,
	coverageMin float64,
	upstreamStepID string,
	commandScope *gatecheck.QualityScopeCommandScope,
) error {
	if commandScope == nil {
		return fmt.Errorf("%w: quality scope command scope is nil", ErrCoverageFailed)
	}
	paths, err := h.resolveCoveragePaths(upstreamStepID)
	if err != nil {
		return err
	}
	return h.runCoveragePipeline(
		ctx, coverageMin, paths, commandScope.CoverageProfileFilter(),
	)
}

func (h *StepRunner) resolveCoveragePaths(upstreamStepID string) (coverageArtifactPaths, error) {
	var out coverageArtifactPaths
	if upstreamStepID == "" {
		return out, fmt.Errorf("%w: upstream step ID not configured", ErrCoverageFailed)
	}
	var err error
	out.rawFileOps, err = h.artifacts.FileOpsPath("coverage.out")
	if err != nil {
		return out, fmt.Errorf("%w: coverage artifact path: %w", ErrCoverageFailed, err)
	}
	out.rawCommand, err = h.artifacts.CommandPath("coverage.out")
	if err != nil {
		return out, fmt.Errorf("%w: coverage argv path: %w", ErrCoverageFailed, err)
	}
	if matErr := h.materializeCoverageProfileForTool(out.rawFileOps, upstreamStepID); matErr != nil {
		return out, matErr
	}
	out.filteredFileOps, err = h.artifacts.FileOpsPath("coverage-filtered.out")
	if err != nil {
		return out, fmt.Errorf("%w: filtered coverage artifact path: %w", ErrCoverageFailed, err)
	}
	out.filteredCommand, err = h.artifacts.CommandPath("coverage-filtered.out")
	if err != nil {
		return out, fmt.Errorf("%w: filtered coverage argv path: %w", ErrCoverageFailed, err)
	}
	return out, nil
}

func (h *StepRunner) runCoveragePipeline(
	ctx context.Context,
	coverageMin float64,
	paths coverageArtifactPaths,
	filter gatecheck.CoverageProfileFilter,
) error {
	activeFileOps, activeCommand, err := h.filterCoverageIfNeeded(
		paths, filter,
	)
	if err != nil {
		return err
	}
	if coverFuncErr := h.runCoverFunc(ctx, activeCommand); coverFuncErr != nil {
		return coverFuncErr
	}
	profileData, err := h.fileOps.ReadFile(activeFileOps)
	if err != nil {
		return fmt.Errorf("%w: read coverage profile: %w", ErrCoverageFailed, err)
	}
	if err := h.checkCoverageResult(profileData, coverageMin); err != nil {
		return err
	}
	// Store the same file we scored (filtered when excludes apply) so artifacts and
	// [go tool cover -func] on the coverage step's output match the threshold check.
	return h.storeCoverageProfileForDownstream(activeFileOps)
}

func (h *StepRunner) filterCoverageIfNeeded(
	paths coverageArtifactPaths,
	filter gatecheck.CoverageProfileFilter,
) (activeFileOps, activeCommand string, err error) {
	if !filter.Needed() {
		return paths.rawFileOps, paths.rawCommand, nil
	}
	data, err := h.fileOps.ReadFile(paths.rawFileOps)
	if err != nil {
		return "", "", fmt.Errorf("%w: read coverage profile: %w", ErrCoverageFailed, err)
	}
	filtered, err := filter.Apply(string(data))
	if err != nil {
		return "", "", fmt.Errorf("%w: filter coverage profile: %w", ErrCoverageFailed, err)
	}
	if !coverageProfileHasDataLines(filtered) {
		return "", "", fmt.Errorf("%w: %w", ErrCoverageFailed, gatecheck.ErrAllPackagesExcluded)
	}
	if err := h.fileOps.WriteFile(paths.filteredFileOps, []byte(filtered), artifactFilePerm); err != nil {
		return "", "", fmt.Errorf("%w: write filtered coverage: %w", ErrCoverageFailed, err)
	}
	return paths.filteredFileOps, paths.filteredCommand, nil
}

func coverageProfileHasDataLines(profile string) bool {
	for _, line := range strings.Split(profile, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		return true
	}
	return false
}

func (h *StepRunner) runCoverFunc(ctx context.Context, coverageCommandPath string) error {
	if _, err := h.runCommand(ctx, h.root, "go", "tool", "cover", "-func="+coverageCommandPath); err != nil {
		return fmt.Errorf("%w: cover-func report: %w", ErrCoverageFailed, err)
	}
	return nil
}

func (h *StepRunner) checkCoverageResult(profileData []byte, coverageMin float64) error {
	result, err := gatecheck.Coverage(string(profileData), coverageMin)
	if err != nil {
		return fmt.Errorf("%w: coverage check: %w", ErrCoverageFailed, err)
	}
	if !result.Passed {
		return &CoverageFailure{result: result}
	}
	return nil
}

func (h *StepRunner) materializeCoverageProfileForTool(coverageFullPath, upstreamStepID string) error {
	coverageData, readErr := h.store.Read(upstreamStepID, "coverage.out")
	if readErr != nil {
		return fmt.Errorf("%w: read coverage from store: %w", ErrCoverageFailed, readErr)
	}
	if mkdirErr := h.fileOps.MkdirAll(h.artifacts.Dir(), artifactDirPerm); mkdirErr != nil {
		return fmt.Errorf("%w: mkdir for coverage: %w", ErrCoverageFailed, mkdirErr)
	}
	if writeErr := h.fileOps.WriteFile(coverageFullPath, coverageData, artifactFilePerm); writeErr != nil {
		return fmt.Errorf("%w: write coverage file: %w", ErrCoverageFailed, writeErr)
	}
	return nil
}

func (h *StepRunner) storeCoverageProfileForDownstream(profilePath string) error {
	coverageData, readErr := h.fileOps.ReadFile(profilePath)
	if readErr != nil {
		return fmt.Errorf("%w: read coverage profile: %w", ErrCoverageFailed, readErr)
	}
	prov := Provenance{StepID: h.stepID, Tool: "go test -coverprofile", Packages: h.packages}
	if writeErr := h.store.Write(h.stepID, "coverage.out", coverageData, prov); writeErr != nil {
		return fmt.Errorf("%w: store coverage profile: %w", ErrCoverageFailed, writeErr)
	}
	return nil
}
