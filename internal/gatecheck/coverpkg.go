// Vision: Build `-coverpkg` lists from quality scope: package expansion, excludes, and test-only filtering.
package gatecheck

import (
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// QualityScopeListFormat is the go-list template used to discover the package inventory
// that seeds coverage and mutation measurement boundaries.
const QualityScopeListFormat = `{{.ImportPath}}` + "\t" + `{{.Dir}}` + "\t" +
	`{{join .TestGoFiles ";"}}` + "\t" +
	`{{join .XTestGoFiles ";"}}` + "\t" +
	`{{if .Module}}{{.Module.Dir}}{{end}}` + "\t" +
	`{{join .GoFiles ";"}}`

// FilterCoverpkg filters import paths by excluding those containing specified segments.
func FilterCoverpkg(importPaths, excludeSegments []string) []string {
	if len(excludeSegments) == 0 {
		return importPaths
	}
	var result []string
	for _, importPath := range importPaths {
		if !isNonProduct(importPath, excludeSegments) {
			result = append(result, importPath)
		}
	}
	return result
}

func isNonProduct(importPath string, excludeSegments []string) bool {
	for _, segment := range excludeSegments {
		if containsSegment(importPath, segment) {
			return true
		}
	}
	return false
}

// excludeSegmentLexical maps '\' to '/' only. Exclude segments come from user/config
// (ParseExcludeSegments, flags); they must not pass through filepath.Clean, which can
// collapse "a/../b" and change substring semantics.
func excludeSegmentLexical(segment string) string {
	return strings.ReplaceAll(segment, `\`, `/`)
}

func containsSegment(path, segment string) bool {
	norm := fsnorm.Canonical(path)
	seg := excludeSegmentLexical(segment)
	if norm == seg {
		return true
	}
	return strings.Contains(norm, "/"+seg+"/") || strings.HasSuffix(norm, "/"+seg)
}

// FilterTestDurations filters test durations by excluding tests whose package import
// paths contain any of the given segments. Uses the same segment-matching rules as FilterCoverpkg.
// Returns the filtered list; returns nil (not an empty slice) when all tests are excluded.
func FilterTestDurations(tests []TestDuration, excludeSegments []string) []TestDuration {
	if len(excludeSegments) == 0 {
		return tests
	}
	result := make([]TestDuration, 0, len(tests))
	for _, test := range tests {
		if !isNonProduct(test.Package, excludeSegments) {
			result = append(result, test)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// PackageImportMatchesExcludeSegment returns true if importPath is excluded
// by a single [Exclude] path segment, using the same rules as [FilterCoverpkg]
// for that segment in isolation.
func PackageImportMatchesExcludeSegment(importPath, segment string) bool {
	return isNonProduct(importPath, []string{segment})
}

// MinimalCoverpkgCSV builds the comma-separated import-path list for -coverpkg after
// applying quality-scope exclude segments. Returns [ErrAllPackagesExcluded] when every
// inventory import path is filtered out.
func MinimalCoverpkgCSV(rows []MutationPackageRow, excludeSegments []string) (string, error) {
	importPaths := make([]string, len(rows))
	for i := range rows {
		importPaths[i] = rows[i].ImportPath
	}
	sort.Strings(importPaths)
	filtered := FilterCoverpkg(importPaths, excludeSegments)
	if len(filtered) == 0 {
		return "", ErrAllPackagesExcluded
	}
	return strings.Join(filtered, ","), nil
}

// ParseExcludeSegments parses a comma-separated string of path segments to exclude.
func ParseExcludeSegments(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := strings.Trim(strings.TrimSpace(part), `/\`)
		if segment == "" {
			continue
		}
		segments = append(segments, excludeSegmentLexical(segment))
	}
	return segments
}
