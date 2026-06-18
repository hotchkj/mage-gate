package gatecheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

const (
	percentMultiplier = 100
)

// CheckMutationCoverageWithScope checks mutation coverage using the structured metrics
// snapshot path, filtering file rows by excludeSegments and testFilePatterns. It returns
// nil on success or an error when coverage falls below minPercent. The structured result
// is available to callers via the snapshot path; this helper returns only the threshold error.
func CheckMutationCoverageWithScope(
	check *MutationKillsCheck,
	minPercent int,
	excludeSegments []string,
	testFilePatterns []string,
) error {
	result, err := CheckMutationCoverageOnMetricsSnapshot(
		SnapshotFromMutationKillsCheck(check), minPercent, excludeSegments, testFilePatterns,
	)
	if err != nil {
		return err
	}
	_ = result // structured result available for callers that need it — intentionally unused here
	return nil
}

// checkMutationCoverageFromScopedFiles enforces the coverage rule on per-file rows using
// the same path filters as [CheckKillsReportSiteBudget]. notCoveredSum is the sum of
// NOT_COVERED counts for included files only.

// forEachScopedNonEmptyMutationFile invokes fn for each per-file row that is included by
// excludeSegments and testFilePatterns and has a non-zero mutation site count. Used by
// [CheckMutationCoverageWithScope] and [CheckKillsReportSiteBudget] so inclusion rules
// stay aligned.
func forEachScopedNonEmptyMutationFile(
	check *MutationKillsCheck,
	excludeSegments []string,
	testFilePatterns []string,
	fn func(row *FileMutationStats, siteCount int),
) {
	for i := range check.Files {
		row := &check.Files[i]
		if shouldSkipMutationFile(row.File, excludeSegments, testFilePatterns) {
			continue
		}
		n := mutationFileSiteCount(row)
		if n == 0 {
			continue
		}
		fn(row, n)
	}
}

func mutationCoverageCountsForFileScope(
	check *MutationKillsCheck,
	excludeSegments []string,
	testFilePatterns []string,
) (total, notCoveredSum int) {
	forEachScopedNonEmptyMutationFile(
		check, excludeSegments, testFilePatterns,
		func(row *FileMutationStats, n int) {
			total += n
			notCoveredSum += row.NotCovered
		},
	)
	return total, notCoveredSum
}

func shouldSkipMutationFile(path string, excludeSegments, testFilePatterns []string) bool {
	if isExcludedPath(path, excludeSegments) {
		return true
	}
	return len(testFilePatterns) > 0 && matchesTestFilePattern(fsnorm.Canonical(path), testFilePatterns)
}

func mutationFileSiteCount(row *FileMutationStats) int {
	return row.Killed + row.Lived + row.NotCovered + row.TimedOut + row.NotViable + row.Runnable
}

// FormatMutationCoverageReport formats per-file mutation coverage for error output.
// Shows files sorted by worst coverage (highest not_covered ratio).
func FormatMutationCoverageReport(check *MutationKillsCheck) string {
	if check == nil || len(check.Files) == 0 {
		return ""
	}

	files := calculateFileCoverage(check)
	if len(files) == 0 {
		return ""
	}

	sortFilesByCoverage(files)

	var sb strings.Builder
	sb.WriteString("Worst coverage files:\n")
	limit := min(len(files), MaxWorstFileRows)
	for i := 0; i < limit; i++ {
		f := files[i]
		fmt.Fprintf(&sb, "  %.1f%%  %s (%d/%d not covered)\n",
			f.percent, f.file, f.notCovered, f.total)
	}
	if len(files) > limit {
		fmt.Fprintf(&sb, "  ... and %d more files\n", len(files)-limit)
	}
	return sb.String()
}

func calculateFileCoverage(check *MutationKillsCheck) []fileCoverage {
	var files []fileCoverage
	for _, row := range check.Files {
		// Skip "unknown" files - they indicate gremlins couldn't extract file paths
		// from the flat mutations format, so they don't provide useful information
		// for identifying worst offenders.
		if row.File == unknownFile {
			continue
		}
		// Keep the report focused on actionable misses; fully covered files add noise.
		if row.NotCovered == 0 {
			continue
		}
		fileTotal := mutationFileSiteCount(&row)
		if fileTotal == 0 {
			continue
		}
		percent := (float64(fileTotal-row.NotCovered) / float64(fileTotal)) * percentMultiplier
		files = append(files, fileCoverage{
			file:       row.File,
			total:      fileTotal,
			notCovered: row.NotCovered,
			percent:    percent,
		})
	}
	return files
}

type fileCoverage struct {
	file       string
	total      int
	notCovered int
	percent    float64
}

func sortFilesByCoverage(files []fileCoverage) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].percent != files[j].percent {
			return files[i].percent < files[j].percent
		}
		return files[i].file < files[j].file
	})
}

// FormatMutationCoverageResultRows formats the structured MutationCoverageResult worst-file rows
// for display in diagnostics. It limits output to reportLimit rows and formats each row as
// "  X.X%  path/to/file.go (N/M not covered)".
func FormatMutationCoverageResultRows(result *MutationCoverageResult, limit int) string {
	if result == nil || len(result.WorstFileRows) == 0 {
		return ""
	}

	if limit <= 0 {
		limit = MaxWorstFileRows
	}

	var sb strings.Builder
	sb.WriteString("Worst coverage files:\n")

	count := min(limit, len(result.WorstFileRows))

	for i := 0; i < count; i++ {
		row := result.WorstFileRows[i]
		fmt.Fprintf(&sb, "  %.1f%%  %s (%d/%d not covered)\n",
			row.Percent, row.File, row.NotCovered, row.Total)
	}

	if len(result.WorstFileRows) > limit {
		fmt.Fprintf(&sb, "  ... and %d more files\n", len(result.WorstFileRows)-limit)
	}

	return sb.String()
}
