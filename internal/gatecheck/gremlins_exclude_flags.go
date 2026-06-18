// Vision: Serialise gremlins --exclude-files= flags from resolved [mutationExcludePath] entries.
package gatecheck

import (
	"regexp"
	"sort"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// globBasenameToRegex converts a basename glob pattern to a regex fragment that allows directory
// separators, for use in package-scoped paths such as ^pkg/.*_test\.go$.
func globBasenameToRegex(glob string) string {
	var builder strings.Builder
	for idx := 0; idx < len(glob); idx++ {
		switch glob[idx] {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '[', ']', '{', '}', '\\':
			builder.WriteByte('\\')
			builder.WriteByte(glob[idx])
		default:
			builder.WriteByte(glob[idx])
		}
	}
	return builder.String()
}

// globBasenameToRootRegex converts a basename glob pattern to a regex fragment that does NOT
// allow directory separators, for use in root-package paths such as ^.*_test\.go$.
func globBasenameToRootRegex(glob string) string {
	var builder strings.Builder
	for idx := 0; idx < len(glob); idx++ {
		switch glob[idx] {
		case '*':
			builder.WriteString("[^/]*")
		case '?':
			builder.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '[', ']', '{', '}', '\\':
			builder.WriteByte('\\')
			builder.WriteByte(glob[idx])
		default:
			builder.WriteByte(glob[idx])
		}
	}
	return builder.String()
}

// packageTestPatternRegex returns an anchored regex matching test files for globPattern inside pkgDir.
// For root packages (pkgDir empty or "."), the regex matches only root-level basenames.
func packageTestPatternRegex(pkgDir, globPattern string) string {
	pkgDir = fsnorm.Canonical(pkgDir)
	globPattern = strings.TrimSpace(globPattern)
	if globPattern == "" {
		return ""
	}
	if pkgDir == "" || pkgDir == "." {
		return "^" + globBasenameToRootRegex(globPattern) + "$"
	}
	return "^" + regexp.QuoteMeta(pkgDir) + "/" + globBasenameToRegex(globPattern) + "$"
}

func fileExcludeFlag(rootRelFile string) string {
	escaped := regexp.QuoteMeta(fsnorm.Canonical(rootRelFile))
	return "--exclude-files=^" + escaped + "$"
}

func dirExcludeFlag(rootRelDir string) string {
	escaped := regexp.QuoteMeta(fsnorm.Canonical(rootRelDir))
	return "--exclude-files=^" + escaped + "(/|$)"
}

func excludeFlag(candidate mutationExcludePath) string {
	if candidate.regex {
		return "--exclude-files=" + candidate.path
	}
	if candidate.dir {
		return dirExcludeFlag(candidate.path)
	}
	return fileExcludeFlag(candidate.path)
}

func serializeGremlinsExcludeFileArgv(excluded map[string]mutationExcludePath) []string {
	return flagsFromExcludedPaths(excluded)
}

func flagsFromExcludedPaths(excluded map[string]mutationExcludePath) []string {
	paths := make([]mutationExcludePath, 0, len(excluded))
	for _, candidate := range excluded {
		paths = append(paths, candidate)
	}
	flags := make([]string, len(paths))
	for idx, candidate := range paths {
		flags[idx] = excludeFlag(candidate)
	}
	sort.Strings(flags)
	return flags
}
