// Vision: Shared gremlins mutation metrics for dry-run and full-run quality checks without re-parsing JSON.
package gatecheck

import (
	"fmt"
	"sort"
)

// MaxWorstFileRows is the maximum number of worst-file rows included in structured
// mutation coverage diagnostics. This constant is shared across gate and gatecheck
// to avoid duplication and ensure consistent behavior.
const MaxWorstFileRows = 10

// MutationFileMetrics is the shared per-file mutation shape used across checks.
type MutationFileMetrics struct {
	File       string
	Killed     int
	Lived      int
	NotCovered int
	TimedOut   int
	NotViable  int
	Runnable   int
}

// MutationMetricsSnapshot carries the parsed metrics once so checks can share one data model.
type MutationMetricsSnapshot struct {
	Files           []MutationFileMetrics
	TotalKilled     int
	TotalLived      int
	TotalNotCovered int
	TotalTimedOut   int
	TotalNotViable  int
	TotalRunnable   int
}

// SnapshotFromMutationKillsCheck bridges existing parsed kill data into the shared snapshot model.
func SnapshotFromMutationKillsCheck(ch *MutationKillsCheck) MutationMetricsSnapshot {
	if ch == nil {
		return MutationMetricsSnapshot{}
	}
	files := make([]MutationFileMetrics, len(ch.Files))
	for i := range ch.Files {
		src := &ch.Files[i]
		files[i] = MutationFileMetrics{
			File:       src.File,
			Killed:     src.Killed,
			Lived:      src.Lived,
			NotCovered: src.NotCovered,
			TimedOut:   src.TimedOut,
			NotViable:  src.NotViable,
			Runnable:   src.Runnable,
		}
	}
	return MutationMetricsSnapshot{
		Files:           files,
		TotalKilled:     ch.TotalKilled,
		TotalLived:      ch.TotalLived,
		TotalNotCovered: ch.TotalNotCovered,
		TotalTimedOut:   ch.TotalTimedOut,
		TotalNotViable:  ch.TotalNotViable,
		TotalRunnable:   ch.TotalRunnable,
	}
}

// FilterMutationMetricsByQualityScope keeps only rows that belong to the caller's quality scope.
func FilterMutationMetricsByQualityScope(
	snap MutationMetricsSnapshot,
	excludeSegments []string,
	testFilePatterns []string,
) MutationMetricsSnapshot {
	var out []MutationFileMetrics
	for _, f := range snap.Files {
		if shouldSkipMutationFile(f.File, excludeSegments, testFilePatterns) {
			continue
		}
		out = append(out, f)
	}
	return MutationMetricsSnapshot{Files: out}
}

// MutationCoverageResult is the structured result returned by CheckMutationCoverageOnMetricsSnapshot
// for curated agent diagnostics. It contains numeric summary and worst-file rows for precise
// ERROR/Fix/Hint construction without text parsing.
type MutationCoverageResult struct {
	// Summary contains the numeric summary for the ERROR message.
	Summary struct {
		Percent        float64
		MinPercent     int
		Covered        int
		Total          int
		NotCovered     int
		ThresholdError error
	}
	// WorstFileRows contains the worst-file rows for the diagnostic output.
	// Each row represents a file with sub-100% coverage, sorted by worst coverage first.
	WorstFileRows []MutationCoverageRow
}

// MutationCoverageRow represents a single file row in the worst-file output.
type MutationCoverageRow struct {
	File       string
	Percent    float64
	Total      int
	NotCovered int
}

// CheckMutationCoverageOnMetricsSnapshot keeps coverage semantics aligned between check and snapshot paths.
// It now returns a structured MutationCoverageResult for curated diagnostics, in addition to the error.
func CheckMutationCoverageOnMetricsSnapshot(
	snap MutationMetricsSnapshot,
	minPercent int,
	excludeSegments []string,
	testFilePatterns []string,
) (*MutationCoverageResult, error) {
	if minPercent <= 0 {
		return nil, nil
	}
	filtered := FilterMutationMetricsByQualityScope(snap, excludeSegments, testFilePatterns)
	return checkMutationCoverageFromScopedFileMetricsList(filtered.Files, minPercent)
}

func checkMutationCoverageFromScopedFileMetricsList(
	files []MutationFileMetrics, minPercent int,
) (*MutationCoverageResult, error) {
	result := &MutationCoverageResult{
		WorstFileRows: make([]MutationCoverageRow, 0),
	}

	// Build worst-file rows from filtered files
	for _, fileMetrics := range files {
		siteCount := mutationFileMetricsSiteCount(&fileMetrics)
		if siteCount == 0 {
			continue
		}
		percent := (float64(siteCount-fileMetrics.NotCovered) / float64(siteCount)) * percentMultiplier
		if fileMetrics.NotCovered > 0 {
			result.WorstFileRows = append(result.WorstFileRows, MutationCoverageRow{
				File:       fileMetrics.File,
				Percent:    percent,
				Total:      siteCount,
				NotCovered: fileMetrics.NotCovered,
			})
		}
	}

	// Sort worst-file rows by worst coverage (lowest percent first)
	sortMutationCoverageRows(result.WorstFileRows)

	// Calculate overall coverage and populate summary.
	checkMutationCoverageSummary(result, files, minPercent)
	return result, result.Summary.ThresholdError
}

// checkMutationCoverageSummary calculates overall coverage and populates the summary.
func checkMutationCoverageSummary(result *MutationCoverageResult, files []MutationFileMetrics, minPercent int) {
	var total, notCoveredSum int
	for _, fileMetrics := range files {
		siteCount := mutationFileMetricsSiteCount(&fileMetrics)
		if siteCount == 0 {
			continue
		}
		total += siteCount
		notCoveredSum += fileMetrics.NotCovered
	}

	if total == 0 {
		err := fmt.Errorf("%w: no mutations in report", ErrMutationCoverageFailed)
		result.Summary = mutationCoverageSummary(0, minPercent, 0, 0, 0, err)
		return
	}

	covered := total - notCoveredSum
	if covered < 0 {
		covered = 0
	}
	percent := (float64(covered) / float64(total)) * percentMultiplier
	result.Summary.Percent = percent
	result.Summary.MinPercent = minPercent
	result.Summary.Covered = covered
	result.Summary.Total = total
	result.Summary.NotCovered = notCoveredSum

	if percent < float64(minPercent) {
		err := fmt.Errorf(
			"%w: mutation coverage %.1f%% below threshold %d%% (%d of %d mutants covered; %d not covered by test profile)",
			ErrMutationCoverageFailed,
			percent,
			minPercent,
			covered,
			total,
			notCoveredSum,
		)
		result.Summary.ThresholdError = err
	}
}

func mutationCoverageSummary(
	percent float64, minPercent, covered, total, notCovered int, err error,
) struct {
	Percent        float64
	MinPercent     int
	Covered        int
	Total          int
	NotCovered     int
	ThresholdError error
} {
	return struct {
		Percent        float64
		MinPercent     int
		Covered        int
		Total          int
		NotCovered     int
		ThresholdError error
	}{
		Percent:        percent,
		MinPercent:     minPercent,
		Covered:        covered,
		Total:          total,
		NotCovered:     notCovered,
		ThresholdError: err,
	}
}

// sortMutationCoverageRows sorts rows by worst coverage (lowest percent first), then by file name.
func sortMutationCoverageRows(rows []MutationCoverageRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Percent != rows[j].Percent {
			return rows[i].Percent < rows[j].Percent
		}
		return rows[i].File < rows[j].File
	})
}

func mutationFileMetricsSiteCount(fileMetrics *MutationFileMetrics) int {
	return fileMetrics.Killed + fileMetrics.Lived + fileMetrics.NotCovered +
		fileMetrics.TimedOut + fileMetrics.NotViable + fileMetrics.Runnable
}
