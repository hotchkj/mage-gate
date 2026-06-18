// Vision: Turn raw coverage.out profiles into line/statement totals and evaluate minimum-percent rules.
package gatecheck

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"

	"golang.org/x/tools/cover"
)

var (
	errParseCoverageProfile = errors.New("parse coverage profile")
	errNoCoverageData       = errors.New("no coverage data in profile")
)

const coverageFullPercent = 100.0

// CoverageResult holds the parsed statement coverage check result.
type CoverageResult struct {
	TotalCoverage     float64
	Passed            bool
	MinCoverage       float64
	CoveredStatements int
	TotalStatements   int
	ThresholdError    error
	WorstFileRows     []CoverageFileRow
}

// CoverageFileRow holds per-file statement coverage data for diagnostics.
type CoverageFileRow struct {
	File              string
	Percent           float64
	CoveredStatements int
	TotalStatements   int
}

// Coverage parses raw coverage.out profile data using golang.org/x/tools/cover and checks against minimum.
// It returns the total coverage percentage and whether the check passed.
// A minCoverage of zero or less disables the threshold: the check passes regardless of the
// reported percentage (parsing must still succeed).
func Coverage(profileData string, minCoverage float64) (CoverageResult, error) {
	profiles, err := cover.ParseProfilesFromReader(strings.NewReader(profileData))
	if err != nil {
		return CoverageResult{}, fmt.Errorf(
			"%w: %w",
			errParseCoverageProfile,
			err,
		)
	}

	if len(profiles) == 0 {
		return CoverageResult{}, errNoCoverageData
	}

	counts := coverageStatementCounts(profiles)
	if counts.total == 0 {
		return CoverageResult{}, errNoCoverageData
	}

	totalCoverage := coveragePercent(counts.covered, counts.total)
	passed := totalCoverage >= minCoverage
	if minCoverage <= 0 {
		passed = true
	}
	var thresholdError error
	if !passed {
		thresholdError = fmt.Errorf(
			"%w: coverage %.1f%% (required >= %.1f%%)",
			ErrCoverageFailed,
			totalCoverage,
			minCoverage,
		)
	}

	return CoverageResult{
		TotalCoverage:     totalCoverage,
		Passed:            passed,
		MinCoverage:       minCoverage,
		CoveredStatements: counts.covered,
		TotalStatements:   counts.total,
		ThresholdError:    thresholdError,
		WorstFileRows:     worstCoverageRows(counts.byFile),
	}, nil
}

type coverageCounts struct {
	covered int
	total   int
	byFile  map[string]coverageFileCounts
}

type coverageFileCounts struct {
	covered int
	total   int
}

func coverageStatementCounts(profiles []*cover.Profile) coverageCounts {
	counts := coverageCounts{byFile: make(map[string]coverageFileCounts, len(profiles))}
	for _, profile := range profiles {
		fileCounts := counts.byFile[profile.FileName]
		for _, block := range profile.Blocks {
			counts.total += block.NumStmt
			fileCounts.total += block.NumStmt
			if block.Count > 0 {
				counts.covered += block.NumStmt
				fileCounts.covered += block.NumStmt
			}
		}
		counts.byFile[profile.FileName] = fileCounts
	}
	return counts
}

func worstCoverageRows(fileCounts map[string]coverageFileCounts) []CoverageFileRow {
	rows := make([]CoverageFileRow, 0, len(fileCounts))
	for file, counts := range fileCounts {
		if counts.total == 0 {
			continue
		}
		percent := coveragePercent(counts.covered, counts.total)
		if percent >= coverageFullPercent {
			continue
		}
		rows = append(rows, CoverageFileRow{
			File:              file,
			Percent:           percent,
			CoveredStatements: counts.covered,
			TotalStatements:   counts.total,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Percent != rows[j].Percent {
			return rows[i].Percent < rows[j].Percent
		}
		return rows[i].File < rows[j].File
	})
	if len(rows) > MaxWorstFileRows {
		return rows[:MaxWorstFileRows]
	}
	return rows
}

func coveragePercent(covered, total int) float64 {
	return 100.0 * float64(covered) / float64(total)
}

// FormatCoverageDiagnosticRows formats worst-file statement coverage rows for diagnostics.
func FormatCoverageDiagnosticRows(result *CoverageResult, limit int) string {
	if result == nil || len(result.WorstFileRows) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = MaxWorstFileRows
	}
	count := min(len(result.WorstFileRows), limit)
	var buf strings.Builder
	buf.WriteString("Worst coverage files:\n")
	for i := 0; i < count; i++ {
		row := result.WorstFileRows[i]
		fmt.Fprintf(
			&buf,
			"  %.1f%%  %s (%d/%d statements covered)\n",
			row.Percent,
			row.File,
			row.CoveredStatements,
			row.TotalStatements,
		)
	}
	if len(result.WorstFileRows) > limit {
		fmt.Fprintf(&buf, "  ... and %d more files\n", len(result.WorstFileRows)-limit)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// CoverageProfileFilter describes post-processing filters for coverage.out profiles.
type CoverageProfileFilter struct {
	ExcludeSegments  []string
	TestFilePatterns []string
}

// Needed reports whether the profile must be filtered before threshold evaluation.
func (f CoverageProfileFilter) Needed() bool {
	return len(f.ExcludeSegments) > 0 || len(f.TestFilePatterns) > 0
}

// Apply filters profileContent when [CoverageProfileFilter.Needed] is true.
func (f CoverageProfileFilter) Apply(profileContent string) (string, error) {
	if !f.Needed() {
		return profileContent, nil
	}
	return FilterCoverageProfile(profileContent, f.ExcludeSegments, f.TestFilePatterns)
}

// FilterCoverageProfile filters a coverage.out file content based on quality-scope filters.
// It reads the coverage profile lines and removes entries that match exclude segments or test file patterns.
// This ensures CRAP checks use the filtered profile for accurate matching.
func FilterCoverageProfile(profileContent string, excludeSegments, testFilePatterns []string) (string, error) {
	if len(excludeSegments) == 0 && len(testFilePatterns) == 0 {
		return profileContent, nil
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(profileContent))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if keepCoverageLine(line, excludeSegments, testFilePatterns) {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan coverage profile: %w", err)
	}

	return buf.String(), nil
}

// keepCoverageLine returns true if the line should be kept (not excluded).
func keepCoverageLine(line string, excludeSegments, testFilePatterns []string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	filename := fsnorm.Canonical(coverProfileLineFilePath(line))
	return !isExcludedPath(filename, excludeSegments) &&
		!matchesTestFilePattern(filename, testFilePatterns)
}

func coverProfileLineFilePath(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) >= 3 && isAllDigits(fields[len(fields)-2]) && isAllDigits(fields[len(fields)-1]) {
		return coverProfileFilePath(strings.Join(fields[:len(fields)-2], " "))
	}
	return coverProfileFilePath(fields[0])
}

func isExcludedPath(filename string, excludeSegments []string) bool {
	for _, segment := range excludeSegments {
		if containsSegment(filename, segment) {
			return true
		}
	}
	return false
}
