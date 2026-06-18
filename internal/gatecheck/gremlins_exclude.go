// Vision: Build deterministic gremlins --exclude-files= argv from [QualityScope] and go list package rows.
package gatecheck

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// MutationPackageRow is one [go list] package line used to derive [BuildGremlinsExcludeArgv].
type MutationPackageRow struct {
	ImportPath      string
	PkgDirRootRel   string
	GoFileNames     []string // non-test .go basenames (from go list GoFiles)
	TestGoFileNames []string
	XTestFileNames  []string
}

// nonTestMutationInventory returns sorted canonical root-relative non-test source paths for rows.
func nonTestMutationInventory(rows []MutationPackageRow) []string {
	seen := make(map[string]struct{})
	for i := range rows {
		pd := fsnorm.Canonical(rows[i].PkgDirRootRel)
		for _, base := range rows[i].GoFileNames {
			base = strings.TrimSpace(base)
			if base == "" {
				continue
			}
			p := fsnorm.Canonical(path.Join(pd, base))
			if p == "" {
				continue
			}
			seen[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

type mutationExcludePath struct {
	path  string
	dir   bool
	regex bool // path is a full anchored exclude regex (e.g. ^pkg/.*_test\.go$)
}

func (p mutationExcludePath) key() string {
	switch {
	case p.regex:
		return "regex:" + p.path
	case p.dir:
		return "dir:" + p.path
	default:
		return "file:" + p.path
	}
}

// pathsExcludedBySegment resolves one [Exclude] segment against the known package and source inventory.
func pathsExcludedBySegment(
	sourceFiles []string,
	rows []MutationPackageRow,
	seg string,
) map[string]mutationExcludePath {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return nil
	}
	segCanon := fsnorm.Canonical(excludeSegmentLexical(seg))
	out := make(map[string]mutationExcludePath)
	addPackageSegmentPathExcludes(out, rows, seg)
	addDirectSegmentFileExcludes(out, sourceFiles, segCanon)
	return out
}

func addDirectSegmentFileExcludes(out map[string]mutationExcludePath, sourceFiles []string, segCanon string) {
	for _, filePath := range sourceFiles {
		if filePath == segCanon {
			addMutationExcludePath(out, mutationExcludePath{path: filePath})
			continue
		}
		if segCanon != "" && strings.HasPrefix(filePath, segCanon+"/") {
			if fileCoveredByDirExclude(filePath, out) {
				continue
			}
			addMutationExcludePath(out, mutationExcludePath{path: filePath})
		}
	}
}

func addPackageSegmentPathExcludes(
	out map[string]mutationExcludePath,
	rows []MutationPackageRow,
	seg string,
) {
	for i := range rows {
		if !PackageImportMatchesExcludeSegment(rows[i].ImportPath, seg) {
			continue
		}
		addMutationExcludePath(out, mutationExcludePath{
			path: fsnorm.Canonical(rows[i].PkgDirRootRel),
			dir:  true,
		})
	}
}

func addMutationExcludePath(out map[string]mutationExcludePath, candidate mutationExcludePath) {
	if candidate.path == "" {
		return
	}
	out[candidate.key()] = candidate
}

func fileCoveredByDirExclude(filePath string, excluded map[string]mutationExcludePath) bool {
	for _, candidate := range excluded {
		if !candidate.dir {
			continue
		}
		if filePath == candidate.path || strings.HasPrefix(filePath, candidate.path+"/") {
			return true
		}
	}
	return false
}

func packageDirExcludedByScope(pkgDir string, excluded map[string]mutationExcludePath) bool {
	pkgDir = fsnorm.Canonical(pkgDir)
	if pkgDir == "" {
		return false
	}
	for _, candidate := range excluded {
		if !candidate.dir {
			continue
		}
		if pkgDir == candidate.path || strings.HasPrefix(pkgDir, candidate.path+"/") {
			return true
		}
	}
	return false
}

func validateTestFilePatterns(patterns []string) error {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, "dummy"); err != nil {
			return fmt.Errorf("%w: %q: %w", ErrInvalidTestFilePattern, pattern, err)
		}
	}
	return nil
}

func anyFileMatchesPattern(names []string, pattern string) bool {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func packageHasTestFilesMatchingPattern(row *MutationPackageRow, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	return anyFileMatchesPattern(row.TestGoFileNames, pattern) ||
		anyFileMatchesPattern(row.XTestFileNames, pattern)
}

func addTestPatternRegexesForPackage(
	out map[string]mutationExcludePath,
	row *MutationPackageRow,
	pd string,
	testFilePatterns []string,
) {
	for _, pattern := range testFilePatterns {
		if !packageHasTestFilesMatchingPattern(row, pattern) {
			continue
		}
		regexText := packageTestPatternRegex(pd, pattern)
		if regexText == "" {
			continue
		}
		candidate := mutationExcludePath{path: regexText, regex: true}
		out[candidate.key()] = candidate
	}
}

func testPatternRegexesForInScopePackages(
	rows []MutationPackageRow,
	alreadyExcluded map[string]mutationExcludePath,
	testFilePatterns []string,
) map[string]mutationExcludePath {
	if len(testFilePatterns) == 0 {
		return nil
	}
	out := make(map[string]mutationExcludePath)
	for rowIdx := range rows {
		pd := fsnorm.Canonical(rows[rowIdx].PkgDirRootRel)
		if pd == "" || packageDirExcludedByScope(pd, alreadyExcluded) {
			continue
		}
		addTestPatternRegexesForPackage(out, &rows[rowIdx], pd, testFilePatterns)
	}
	return out
}

func mutationPathsExcludedByScope(
	inventory []string,
	sourceFiles []string,
	rows []MutationPackageRow,
	excludeSegments, testFilePatterns []string,
) map[string]mutationExcludePath {
	excluded := make(map[string]mutationExcludePath)
	segmentInventory := canonicalSourceFilesOrInventory(sourceFiles, inventory)
	for _, seg := range excludeSegments {
		for key, candidate := range pathsExcludedBySegment(segmentInventory, rows, seg) {
			excluded[key] = candidate
		}
	}
	promoteSegmentFileExcludesToDirExcludes(excluded, excludeSegments, inventory)
	addPatternExcludedMutationPaths(excluded, inventory, testFilePatterns)
	for key, candidate := range testPatternRegexesForInScopePackages(rows, excluded, testFilePatterns) {
		excluded[key] = candidate
	}
	pruneCoveredChildDirExcludes(excluded)
	return excluded
}

// promoteSegmentFileExcludesToDirExcludes replaces per-file excludes under an exclude segment
// with a single directory exclude when no in-scope production file from the non-test inventory
// lives under that segment prefix. This is the global minimization pass that prevents
// file enumeration when a segment safely covers the entire subtree.
func promoteSegmentFileExcludesToDirExcludes(
	excluded map[string]mutationExcludePath,
	excludeSegments []string,
	inventory []string,
) {
	for _, seg := range excludeSegments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		segCanon := fsnorm.Canonical(excludeSegmentLexical(seg))
		if segCanon == "" {
			continue
		}
		tryPromoteSegmentToDirExclude(excluded, segCanon, inventory)
	}
}

func tryPromoteSegmentToDirExclude(
	excluded map[string]mutationExcludePath,
	segCanon string,
	inventory []string,
) {
	if hasDirExcludeForSegment(excluded, segCanon) {
		return
	}
	if inventoryHasInScopeFileUnderSegment(inventory, segCanon) {
		return
	}
	fileKeys := fileExcludeKeysUnderSegment(excluded, segCanon)
	if len(fileKeys) == 0 {
		return
	}
	if !segmentHasChildFileExcludes(excluded, fileKeys, segCanon) {
		return
	}
	for _, key := range fileKeys {
		delete(excluded, key)
	}
	addMutationExcludePath(excluded, mutationExcludePath{path: segCanon, dir: true})
}

func hasDirExcludeForSegment(excluded map[string]mutationExcludePath, segCanon string) bool {
	for _, candidate := range excluded {
		if candidate.dir && (candidate.path == segCanon || strings.HasPrefix(segCanon, candidate.path+"/")) {
			return true
		}
	}
	return false
}

func inventoryHasInScopeFileUnderSegment(inventory []string, segCanon string) bool {
	for _, f := range inventory {
		if f == segCanon || strings.HasPrefix(f, segCanon+"/") {
			return true
		}
	}
	return false
}

func fileExcludeKeysUnderSegment(excluded map[string]mutationExcludePath, segCanon string) []string {
	var keys []string
	for key, candidate := range excluded {
		if candidate.dir || candidate.regex {
			continue
		}
		if candidate.path == segCanon || strings.HasPrefix(candidate.path, segCanon+"/") {
			keys = append(keys, key)
		}
	}
	return keys
}

// segmentHasChildFileExcludes returns true only when at least one file exclude
// is a child of segCanon (not segCanon itself). A segment that matches only its
// own path is a single-file exclude and must not be promoted to a directory regex.
func segmentHasChildFileExcludes(
	excluded map[string]mutationExcludePath,
	fileKeys []string,
	segCanon string,
) bool {
	for _, key := range fileKeys {
		candidate := excluded[key]
		if candidate.path != segCanon {
			return true
		}
	}
	return false
}

func canonicalSourceFilesOrInventory(sourceFiles, inventory []string) []string {
	if len(sourceFiles) == 0 {
		return inventory
	}
	out := make([]string, 0, len(sourceFiles))
	for _, filePath := range sourceFiles {
		filePath = fsnorm.Canonical(filePath)
		if filePath != "" {
			out = append(out, filePath)
		}
	}
	sort.Strings(out)
	return out
}

func addPatternExcludedMutationPaths(
	excluded map[string]mutationExcludePath,
	inventory []string,
	testFilePatterns []string,
) {
	if len(testFilePatterns) == 0 {
		return
	}
	for _, f := range inventory {
		if matchesTestFilePattern(f, testFilePatterns) {
			addMutationExcludePath(excluded, mutationExcludePath{path: f})
		}
	}
}

func pruneCoveredChildDirExcludes(excluded map[string]mutationExcludePath) {
	for key, candidate := range excluded {
		if !candidate.dir {
			continue
		}
		for otherKey, other := range excluded {
			if key == otherKey || !other.dir {
				continue
			}
			if strings.HasPrefix(candidate.path, other.path+"/") {
				delete(excluded, key)
				break
			}
		}
	}
}

func allInventoryPathsExcluded(inventory []string, excluded map[string]mutationExcludePath) bool {
	if len(inventory) == 0 {
		return false
	}
	for _, f := range inventory {
		if !mutationFileExcluded(f, excluded) {
			return false
		}
	}
	return true
}

func mutationFileExcluded(filePath string, excluded map[string]mutationExcludePath) bool {
	filePath = fsnorm.Canonical(filePath)
	for _, candidate := range excluded {
		if candidate.dir {
			if filePath == candidate.path || strings.HasPrefix(filePath, candidate.path+"/") {
				return true
			}
			continue
		}
		if filePath == candidate.path {
			return true
		}
	}
	return false
}

func resolveMutationExcludePaths(
	rows []MutationPackageRow,
	sourceFiles []string,
	excludeSegments, testFilePatterns []string,
) (inventory []string, excluded map[string]mutationExcludePath) {
	inventory = nonTestMutationInventory(rows)
	excluded = mutationPathsExcludedByScope(inventory, sourceFiles, rows, excludeSegments, testFilePatterns)
	return inventory, excluded
}

// BuildGremlinsExcludeArgv returns full gremlins arguments "--exclude-files=^…$" in sorted order.
// Exclusion is computed from a canonical non-test inventory plus package-level test-pattern regexes;
// directory/package excludes resolve to canonical path regexes before serialization. sourceFiles is the
// complete root-relative .go file list visible to gremlins; path-segment excludes match that source list so
// files such as testdata helpers can be excluded even when they are not go-list packages.
// If every non-test source path would be excluded (segments and/or test patterns applied to inventory),
// it returns [ErrAllPackagesExcluded]. When go list reports no non-test files, only concrete source-file
// and test-pattern excludes can be emitted.
func BuildGremlinsExcludeArgv(
	rows []MutationPackageRow,
	sourceFiles []string,
	excludeSegments, testFilePatterns []string,
) ([]string, error) {
	if err := validateTestFilePatterns(testFilePatterns); err != nil {
		return nil, err
	}
	inv, excluded := resolveMutationExcludePaths(rows, sourceFiles, excludeSegments, testFilePatterns)
	if allInventoryPathsExcluded(inv, excluded) {
		return nil, ErrAllPackagesExcluded
	}
	return serializeGremlinsExcludeFileArgv(excluded), nil
}

// GremlinsExcludeFileArgs is a convenience wrapper that drops [ErrAllPackagesExcluded]
// (returns nil argv). Other errors, including [ErrInvalidTestFilePattern], propagate.
// Callers that must fail closed should use [BuildGremlinsExcludeArgv].
func GremlinsExcludeFileArgs(
	rows []MutationPackageRow,
	excludeSegments, testFilePatterns []string,
) ([]string, error) {
	out, err := BuildGremlinsExcludeArgv(rows, nil, excludeSegments, testFilePatterns)
	if errors.Is(err, ErrAllPackagesExcluded) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}
