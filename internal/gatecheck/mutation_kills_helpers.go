// Vision: Small parsers/normalizers shared by mutation kill-rate logic (package names, sorting, string helpers).
package gatecheck

import (
	"fmt"
	"sort"
	"strings"
)

// normalizePackageName ensures missing/empty package names are normalized to "unknown".
func normalizePackageName(pkg string) string {
	if pkg == "" {
		return unknownPackage
	}
	return pkg
}

// normalizeStatus validates and normalizes mutation status strings.
// Trims whitespace and performs case-insensitive matching.
// Returns an error for unknown or missing statuses.
func normalizeStatus(status string) (string, error) {
	normalized := strings.TrimSpace(strings.ToUpper(status))
	// Gremlins JSON may use spaced labels (e.g. "TIMED OUT") while gatecheck uses underscores.
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if normalized == "" {
		return "", fmt.Errorf("%w", errEmptyStatus)
	}

	validStatuses := map[string]bool{
		statusKilled:     true,
		statusLived:      true,
		statusNotCovered: true,
		statusTimedOut:   true,
		statusNotViable:  true,
		statusRunnable:   true,
	}

	if !validStatuses[normalized] {
		return "", fmt.Errorf("%w: %q", errUnknownStatus, status)
	}

	return normalized, nil
}

// getOrCreateFileStats returns existing or creates new file stats entry.
func getOrCreateFileStats(m map[string]*FileMutationStats, file string) *FileMutationStats {
	if stats, ok := m[file]; ok {
		return stats
	}
	stats := &FileMutationStats{File: file}
	m[file] = stats
	return stats
}

// getOrCreatePackageStats returns existing or creates new package stats entry.
func getOrCreatePackageStats(m map[string]*PackageMutationStats, pkg string) *PackageMutationStats {
	if stats, ok := m[pkg]; ok {
		return stats
	}
	stats := &PackageMutationStats{Package: pkg}
	m[pkg] = stats
	return stats
}

// incrementFileStatusCounts increments the appropriate status counter for a file.
func incrementFileStatusCounts(fileStats *FileMutationStats, status string) {
	switch status {
	case statusKilled:
		fileStats.Killed++
	case statusLived:
		fileStats.Lived++
	case statusNotCovered:
		fileStats.NotCovered++
	case statusTimedOut:
		fileStats.TimedOut++
	case statusNotViable:
		fileStats.NotViable++
	case statusRunnable:
		fileStats.Runnable++
	}
}

// incrementPackageStatusCounts increments the appropriate status counter for a package.
func incrementPackageStatusCounts(pkgStats *PackageMutationStats, status string) {
	switch status {
	case statusKilled:
		pkgStats.Killed++
	case statusLived:
		pkgStats.Lived++
	case statusNotCovered:
		pkgStats.NotCovered++
	case statusTimedOut:
		pkgStats.TimedOut++
	case statusNotViable:
		pkgStats.NotViable++
	case statusRunnable:
		pkgStats.Runnable++
	}
}

// incrementGlobalCounts increments the appropriate global status counter.
func incrementGlobalCounts(check *MutationKillsCheck, status string) {
	switch status {
	case statusKilled:
		check.TotalKilled++
	case statusLived:
		check.TotalLived++
	case statusNotCovered:
		check.TotalNotCovered++
	case statusTimedOut:
		check.TotalTimedOut++
	case statusNotViable:
		check.TotalNotViable++
	case statusRunnable:
		check.TotalRunnable++
	}
}

// computeKillRate calculates the kill rate percentage.
func computeKillRate(check *MutationKillsCheck) {
	denominator := check.TotalKilled + check.TotalLived
	if denominator > 0 {
		check.KillRatePercent = (float64(check.TotalKilled) / float64(denominator)) * killRatePercent
	}
}

// sortFileStats sorts file statistics by file path.
func sortFileStats(m map[string]*FileMutationStats) []FileMutationStats {
	stats := make([]FileMutationStats, 0, len(m))
	for _, s := range m {
		stats = append(stats, *s)
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].File < stats[j].File
	})
	return stats
}

// sortPackageStats sorts package statistics by package path.
func sortPackageStats(m map[string]*PackageMutationStats) []PackageMutationStats {
	stats := make([]PackageMutationStats, 0, len(m))
	for _, s := range m {
		stats = append(stats, *s)
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Package < stats[j].Package
	})
	return stats
}

const maxTimedOutDetailsPerFile = 12

// maxLivedAllowedForMinKillRate is the largest integer L such that, for fixed killed and
// 0 < minRate < killRatePercentScale, kill rate killed/(killed+L) can still reach minRate% when lived ≤ L.
// Equivalently: L_max = floor(killed * (scale-minRate) / minRate). For minRate ≥ scale, returns 0.
func maxLivedAllowedForMinKillRate(killed, minRate int) int {
	if killed < 0 || minRate <= 0 {
		return 0
	}
	if minRate >= killRatePercentScale {
		return 0
	}
	return killed * (killRatePercentScale - minRate) / minRate
}

func appendTimedOutMutationDetail(fs *FileMutationStats, mut map[string]any) {
	if fs == nil || len(fs.TimedOutDetails) >= maxTimedOutDetailsPerFile {
		return
	}
	fs.TimedOutDetails = append(fs.TimedOutDetails, formatTimedOutMutationDetail(mut))
}

func formatTimedOutMutationDetail(mut map[string]any) string {
	typ, _ := mut["type"].(string)
	typ = strings.TrimSpace(typ)
	if typ == "" {
		typ = "UNKNOWN_TYPE"
	}
	line := jsonNumberToInt(mut["line"])
	col := jsonNumberToInt(mut["column"])
	if line > 0 || col > 0 {
		return fmt.Sprintf("%s line %d col %d", typ, line, col)
	}
	return typ
}

func jsonNumberToInt(v any) int {
	switch num := v.(type) {
	case int:
		return num
	case int64:
		return int(num)
	case float64:
		return int(num)
	default:
		return 0
	}
}
