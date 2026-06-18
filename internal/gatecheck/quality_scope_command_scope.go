// Vision: One shared quality-scope command scope translates inventory and scope into minimal tool inputs.
package gatecheck

import (
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// QualityScopeCommandScope is the canonical internal translation of quality scope into
// command inputs and post-processing filters. Exclude segments are parsed once at
// construction; consumers must use command-scope projections instead of reparsing raw scope.
type QualityScopeCommandScope struct {
	rows                    []MutationPackageRow
	sourceFiles             []string
	sourceInventoryGathered bool
	excludeSegments         []string
	testFilePatterns        []string
	tags                    []string
}

// NewQualityScopeCommandScope builds command inputs from package inventory rows, an optional
// canonical root-relative .go source inventory, raw exclude-segment string, test file patterns,
// and build tags. sourceFiles may be nil when callers only need coverpkg, coverage-profile,
// CRAP, or threshold-filter projections; mutation exclude-file projections require sourceFiles
// when exclude segments do not match any package import path.
func NewQualityScopeCommandScope(
	rows []MutationPackageRow,
	sourceFiles []string,
	rawExcludeSegments string,
	testFilePatterns, tags []string,
) QualityScopeCommandScope {
	canonical := canonicalSourceFilesOrInventory(sourceFiles, nil)
	return QualityScopeCommandScope{
		rows:                    append([]MutationPackageRow(nil), rows...),
		sourceFiles:             canonical,
		sourceInventoryGathered: sourceFiles != nil,
		excludeSegments:         ParseExcludeSegments(rawExcludeSegments),
		testFilePatterns:        append([]string(nil), testFilePatterns...),
		tags:                    append([]string(nil), tags...),
	}
}

// Tags returns quality-scope build tags in stable order.
func (s *QualityScopeCommandScope) Tags() []string {
	return append([]string(nil), s.tags...)
}

// TagsCSV returns build tags joined for -tags= / --tags= argv emission.
func (s *QualityScopeCommandScope) TagsCSV() string {
	return strings.Join(s.tags, ",")
}

// ExcludeSegments returns parsed exclude path segments for threshold path filters.
func (s *QualityScopeCommandScope) ExcludeSegments() []string {
	return append([]string(nil), s.excludeSegments...)
}

// TestFilePatterns returns configured test-file patterns for threshold path filters.
func (s *QualityScopeCommandScope) TestFilePatterns() []string {
	return append([]string(nil), s.testFilePatterns...)
}

// ThresholdPathFilters returns the parsed exclude segments and test file patterns shared
// by coverage-profile filtering, CRAP output parsing, and mutation threshold checks.
func (s *QualityScopeCommandScope) ThresholdPathFilters() (excludeSegments, testFilePatterns []string) {
	return s.ExcludeSegments(), s.TestFilePatterns()
}

// CoverpkgCSV returns the minimal comma-separated import-path list for -coverpkg.
func (s *QualityScopeCommandScope) CoverpkgCSV() (string, error) {
	return MinimalCoverpkgCSV(s.rows, s.excludeSegments)
}

// CoverageProfileFilter returns the profile post-processing filter for the coverage step.
func (s *QualityScopeCommandScope) CoverageProfileFilter() CoverageProfileFilter {
	return CoverageProfileFilter{
		ExcludeSegments:  append([]string(nil), s.excludeSegments...),
		TestFilePatterns: append([]string(nil), s.testFilePatterns...),
	}
}

// GocycloPkgDirsRootRel returns sorted root-relative package directories for in-scope
// packages after package-level excludes. Host path resolution remains with callers.
func (s *QualityScopeCommandScope) GocycloPkgDirsRootRel() ([]string, error) {
	filtered, err := s.filteredRowsByImport()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(filtered))
	dirs := make([]string, 0, len(filtered))
	for i := range filtered {
		dir := fsnorm.Canonical(filtered[i].PkgDirRootRel)
		if dir == "" {
			dir = "."
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		return nil, ErrAllPackagesExcluded
	}
	return dirs, nil
}

// WithSourceInventory returns an equivalent command scope with canonical root-relative .go source paths
// for mutation exclude-file projections.
func (s *QualityScopeCommandScope) WithSourceInventory(sourceFiles []string) QualityScopeCommandScope {
	out := *s
	out.sourceFiles = canonicalSourceFilesOrInventory(sourceFiles, nil)
	out.sourceInventoryGathered = true
	return out
}

// MutationExcludeFileArgv returns minimized gremlins --exclude-files= argv in sorted order.
// Package-level exclude segments resolve from inventory rows alone; path-segment excludes that
// do not match any package import require the root-relative .go source inventory from a walk.
func (s *QualityScopeCommandScope) MutationExcludeFileArgv() ([]string, error) {
	if mutationExcludeRequiresSourceInventory(s.rows, s.excludeSegments) && !s.sourceInventoryGathered {
		return nil, ErrQualityScopeSourceInventoryRequired
	}
	return BuildGremlinsExcludeArgv(
		s.rows,
		s.mutationExcludeSourceFiles(),
		s.excludeSegments,
		s.testFilePatterns,
	)
}

func (s *QualityScopeCommandScope) mutationExcludeSourceFiles() []string {
	if !mutationExcludeRequiresSourceInventory(s.rows, s.excludeSegments) {
		return nil
	}
	return s.sourceFiles
}

// mutationExcludeRequiresSourceInventory reports whether gremlins exclude argv needs a walked
// root-relative .go source list (for example testdata paths outside go list packages).
func mutationExcludeRequiresSourceInventory(rows []MutationPackageRow, excludeSegments []string) bool {
	for _, seg := range excludeSegments {
		if !excludeSegmentMatchesPackageInventory(rows, seg) {
			return true
		}
	}
	return false
}

func excludeSegmentMatchesPackageInventory(rows []MutationPackageRow, seg string) bool {
	for i := range rows {
		if PackageImportMatchesExcludeSegment(rows[i].ImportPath, seg) {
			return true
		}
	}
	return false
}

func (s *QualityScopeCommandScope) filteredRowsByImport() ([]MutationPackageRow, error) {
	filteredImports, err := s.filteredImportPaths()
	if err != nil {
		return nil, err
	}
	importSet := make(map[string]struct{}, len(filteredImports))
	for _, importPath := range filteredImports {
		importSet[importPath] = struct{}{}
	}
	out := make([]MutationPackageRow, 0, len(filteredImports))
	for i := range s.rows {
		if _, ok := importSet[s.rows[i].ImportPath]; ok {
			out = append(out, s.rows[i])
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ImportPath < out[j].ImportPath
	})
	return out, nil
}

func (s *QualityScopeCommandScope) filteredImportPaths() ([]string, error) {
	importPaths := make([]string, len(s.rows))
	for i := range s.rows {
		importPaths[i] = s.rows[i].ImportPath
	}
	sort.Strings(importPaths)
	filtered := FilterCoverpkg(importPaths, s.excludeSegments)
	if len(filtered) == 0 {
		return nil, ErrAllPackagesExcluded
	}
	return filtered, nil
}
