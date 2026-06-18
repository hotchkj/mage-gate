// Vision: Gremlins JSON → per-file site totals, sorted reporting, and cap checks independent of subprocesses.
package gatecheck

import (
	"fmt"
	"sort"
	"strings"
)

// MutationPair represents a file and its mutation site count.
type MutationPair struct {
	Path  string
	Count int
}

// MutationResult holds the mutation site check result.
type MutationResult struct {
	PerFile  []MutationPair
	MaxSites int
	Passed   bool
}

// CountMutationsPerFile parses gremlins JSON output and counts mutations per file.
func CountMutationsPerFile(jsonData []byte) (map[string]int, error) {
	root, err := parseGremlinsMutationRoot(jsonData)
	if err != nil {
		return nil, fmt.Errorf("parse mutations JSON: %w", err)
	}
	return countMutationSitesFromRoot(root)
}

// MutationSites checks mutation site counts against the maximum.
// maxSites must be > 0; a value of 0 or less is a configuration error.
func MutationSites(jsonData []byte, maxSites int) (MutationResult, error) {
	if maxSites <= 0 {
		return MutationResult{}, fmt.Errorf("%w: got %d", errInvalidMaxSites, maxSites)
	}
	perFile, err := CountMutationsPerFile(jsonData)
	if err != nil {
		return MutationResult{}, err
	}
	list := sortMutationCounts(perFile)
	offenders := collectMutationOffenders(list, maxSites)
	return MutationResult{
		PerFile:  list,
		MaxSites: maxSites,
		Passed:   len(offenders) == 0,
	}, nil
}

func sortMutationCounts(perFile map[string]int) []MutationPair {
	list := make([]MutationPair, 0, len(perFile))
	for path, count := range perFile {
		list = append(list, MutationPair{Path: path, Count: count})
	}
	sort.Slice(list, func(left, right int) bool {
		if list[left].Count != list[right].Count {
			return list[left].Count > list[right].Count
		}
		return list[left].Path < list[right].Path
	})
	return list
}

func collectMutationOffenders(list []MutationPair, maxSites int) []MutationPair {
	offenders := make([]MutationPair, 0, len(list))
	for _, entry := range list {
		if entry.Count > maxSites {
			offenders = append(offenders, entry)
		}
	}
	return offenders
}

// CheckKillsReportSiteBudget enforces a per-file mutation site cap from a [MutationKillsCheck]
// produced by the same gremlins JSON pipeline as [ParseMutationKillsReport] (per-file totals
// match site counting). File paths are filtered with excludeSegments and testFilePatterns
// using the same path rules as [FilterCoverageProfile] and [Crap] test-file skipping.
// maxSites must be positive, matching [MutationSites] on raw JSON.
func CheckKillsReportSiteBudget(
	check *MutationKillsCheck,
	maxSites int,
	excludeSegments []string,
	testFilePatterns []string,
) error {
	if maxSites <= 0 {
		return fmt.Errorf("%w: got %d", errInvalidMaxSites, maxSites)
	}
	perFile := make(map[string]int, len(check.Files))
	forEachScopedNonEmptyMutationFile(
		check, excludeSegments, testFilePatterns,
		func(row *FileMutationStats, n int) {
			perFile[row.File] = n
		},
	)
	list := sortMutationCounts(perFile)
	offenders := collectMutationOffenders(list, maxSites)
	if len(offenders) == 0 {
		return nil
	}
	result := MutationResult{
		PerFile:  list,
		MaxSites: maxSites,
		Passed:   false,
	}
	return fmt.Errorf("%w: %s", ErrMutationSitesFailed, FormatMutationReport(result))
}

// FormatMutationReport formats the mutation site check result for output.
func FormatMutationReport(result MutationResult) string {
	if result.Passed {
		return fmt.Sprintf("Mutation site counts OK (none above %d per file).", result.MaxSites)
	}
	var sb strings.Builder
	fmt.Fprintf(
		&sb,
		"Files with more than %d mutation sites (rule: split by theme, move >=30%% of code out):\n",
		result.MaxSites,
	)
	for _, entry := range result.PerFile {
		if entry.Count > result.MaxSites {
			fmt.Fprintf(&sb, "  %4d  %s\n", entry.Count, entry.Path)
		}
	}
	return sb.String()
}
