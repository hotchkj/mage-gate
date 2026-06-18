// Vision: Decode gremlins full-run JSON into per-package kill stats for threshold checks (no subprocess).
package gatecheck

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// Mutation status constants for consistent handling.
const (
	statusKilled         = "KILLED"
	statusLived          = "LIVED"
	statusNotCovered     = "NOT_COVERED"
	statusTimedOut       = "TIMED_OUT"
	statusNotViable      = "NOT_VIABLE"
	statusRunnable       = "RUNNABLE"
	unknownPackage       = "unknown"
	killRatePercent      = 100.0
	killRatePercentScale = 100
)

// MutationKillsCheck holds the mutation kill rate check result.
type MutationKillsCheck struct {
	TotalKilled     int
	TotalLived      int
	TotalNotCovered int
	TotalTimedOut   int
	TotalNotViable  int
	TotalRunnable   int
	KillRatePercent float64
	Files           []FileMutationStats
	Packages        []PackageMutationStats
}

// FileMutationStats represents mutation status counts per file.
type FileMutationStats struct {
	File            string
	Killed          int
	Lived           int
	NotCovered      int
	TimedOut        int
	NotViable       int
	Runnable        int
	TimedOutDetails []string // Gremlins mutation summaries (type/line/col), capped per file
}

// PackageMutationStats represents mutation status counts per package.
type PackageMutationStats struct {
	Package    string
	Killed     int
	Lived      int
	NotCovered int
	TimedOut   int
	NotViable  int
	Runnable   int
}

// ParseMutationKillsReport parses gremlins JSON and returns detailed stats.
func ParseMutationKillsReport(r io.Reader) (*MutationKillsCheck, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read gremlins output: %w", err)
	}

	root, err := parseGremlinsMutationRoot(data)
	if err != nil {
		return nil, err
	}

	check, err := buildMutationKillsCheckFromRoot(root)
	if err != nil {
		return nil, err
	}

	computeKillRate(check)
	return check, nil
}

// CheckMutationKillsRate validates kill rate against min threshold.
// MinRate(0) disables threshold check and always passes.
// If MinRate > 0 and denominator == 0, return error.
// For enabled threshold, pass when actualRate >= minRate.
func CheckMutationKillsRate(check *MutationKillsCheck, minRate int) error {
	if minRate <= 0 {
		return nil
	}

	denominator := check.TotalKilled + check.TotalLived
	if denominator == 0 {
		return fmt.Errorf("%w: no killed or lived mutations to evaluate kill rate", ErrMutationKillsFailed)
	}

	if check.KillRatePercent < float64(minRate) {
		return fmt.Errorf("%w: kill rate %.1f%% below threshold %d%%",
			ErrMutationKillsFailed, check.KillRatePercent, minRate)
	}

	return nil
}

// CheckMutationCoverage enforces a minimum share of mutants with test coverage (any status
// other than NOT_COVERED) among all mutants in the gremlins report. A minPercent of 0
// disables the check (always passes). When minPercent is positive and the report
// contains no mutations, the check fails.
func CheckMutationCoverage(check *MutationKillsCheck, minPercent int) error {
	if minPercent <= 0 {
		return nil
	}
	total := check.TotalKilled + check.TotalLived + check.TotalNotCovered +
		check.TotalTimedOut + check.TotalNotViable + check.TotalRunnable
	if total == 0 {
		return fmt.Errorf("%w: no mutations in report", ErrMutationCoverageFailed)
	}
	covered := total - check.TotalNotCovered
	if covered < 0 {
		covered = 0
	}
	percent := (float64(covered) / float64(total)) * killRatePercent
	if percent < float64(minPercent) {
		coverageReport := FormatMutationCoverageReport(check)
		if coverageReport == "" {
			return fmt.Errorf(
				"%w: mutation coverage %.1f%% below threshold %d%% (%d of %d mutants covered; %d not covered by test profile)",
				ErrMutationCoverageFailed,
				percent,
				minPercent,
				covered,
				total,
				check.TotalNotCovered,
			)
		}
		return fmt.Errorf(
			"%w: mutation coverage %.1f%% below threshold %d%% (%d of %d mutants covered; %d not covered by test profile)\n%s",
			ErrMutationCoverageFailed,
			percent,
			minPercent,
			covered,
			total,
			check.TotalNotCovered,
			coverageReport,
		)
	}
	return nil
}

// buildMutationKillsCheckFromRoot aggregates kill stats using the same file/flat decode as site counting.
func buildMutationKillsCheckFromRoot(root map[string]json.RawMessage) (*MutationKillsCheck, error) {
	check := &MutationKillsCheck{
		Files:    []FileMutationStats{},
		Packages: []PackageMutationStats{},
	}
	fileStatsMap := make(map[string]*FileMutationStats)
	pkgStatsMap := make(map[string]*PackageMutationStats)

	filesRaw, useFiles, selErr := selectGremlinsFilesRaw(root)
	if selErr != nil {
		return nil, selErr
	}
	if useFiles {
		bundles, err := parseGremlinsFileBundles(filesRaw)
		if err != nil {
			return nil, err
		}
		if err := aggregateKillsFromGremlinsFileBundles(bundles, check, fileStatsMap, pkgStatsMap); err != nil {
			return nil, err
		}
	} else {
		muts, err := parseGremlinsFlatMutationMaps(root)
		if err != nil {
			return nil, err
		}
		if err := aggregateKillsFromGremlinsFlatMuts(muts, check, fileStatsMap, pkgStatsMap); err != nil {
			return nil, err
		}
	}

	check.Files = sortFileStats(fileStatsMap)
	check.Packages = sortPackageStats(pkgStatsMap)
	return check, nil
}

func aggregateKillsFromGremlinsFileBundles(
	bundles []gremlinsFileBundle,
	check *MutationKillsCheck,
	fileStatsMap map[string]*FileMutationStats,
	pkgStatsMap map[string]*PackageMutationStats,
) error {
	for _, bundle := range bundles {
		// Match prior behavior: every files[] entry gets file/package rows even when mutations is empty.
		fileStats := getOrCreateFileStats(fileStatsMap, bundle.File)
		pkgStats := getOrCreatePackageStats(pkgStatsMap, bundle.Package)
		for _, mut := range bundle.Mutations {
			status, ok := mut["status"].(string)
			if !ok {
				return fmt.Errorf("%w for file %q", errMissingStatus, bundle.File)
			}
			normalized, nerr := normalizeStatus(status)
			if nerr != nil {
				return fmt.Errorf("file %q: %w", bundle.File, nerr)
			}
			incrementFileStatusCounts(fileStats, normalized)
			incrementPackageStatusCounts(pkgStats, normalized)
			incrementGlobalCounts(check, normalized)
			if normalized == statusTimedOut {
				appendTimedOutMutationDetail(fileStats, mut)
			}
		}
	}
	return nil
}

func aggregateKillsFromGremlinsFlatMuts(
	muts []map[string]any,
	check *MutationKillsCheck,
	fileStatsMap map[string]*FileMutationStats,
	pkgStatsMap map[string]*PackageMutationStats,
) error {
	for _, mut := range muts {
		if err := processFlatMutationKills(mut, check, fileStatsMap, pkgStatsMap); err != nil {
			return err
		}
	}
	return nil
}

func processFlatMutationKills(
	mut map[string]any,
	check *MutationKillsCheck,
	fileStatsMap map[string]*FileMutationStats,
	pkgStatsMap map[string]*PackageMutationStats,
) error {
	status, ok := mut["status"].(string)
	if !ok {
		return fmt.Errorf("%w", errMissingStatus)
	}

	normalized, err := normalizeStatus(status)
	if err != nil {
		return err
	}

	fileName := resolveFileName(mut)
	pkg := resolvePackage(mut)

	fileStats := getOrCreateFileStats(fileStatsMap, fileName)
	pkgStats := getOrCreatePackageStats(pkgStatsMap, pkg)

	incrementFileStatusCounts(fileStats, normalized)
	incrementPackageStatusCounts(pkgStats, normalized)
	incrementGlobalCounts(check, normalized)
	if normalized == statusTimedOut {
		appendTimedOutMutationDetail(fileStats, mut)
	}

	return nil
}

func resolveFileName(mut map[string]any) string {
	fileName := firstNonEmptyStringField(mut, "file", "filename")
	if fileName == "" {
		fileName = unknownFile
	}
	return fsnorm.Canonical(fileName)
}

func resolvePackage(mut map[string]any) string {
	pkg := stringField(mut, "package")
	return normalizePackageName(pkg)
}

// MutationKillsResult wraps the check with structured result information.
type MutationKillsResult struct {
	Check              *MutationKillsCheck
	Passed             bool
	MinKillRatePercent int // required threshold from the caller (0 = disabled)
	Denominator        int
	MaxLivedAllowed    int
	LivedOverBudget    int
	ThresholdError     error
}

// EvaluateMutationKills evaluates a parsed mutation report against the required kill rate.
func EvaluateMutationKills(check *MutationKillsCheck, minKillRate int) MutationKillsResult {
	result := MutationKillsResult{
		Check:              check,
		MinKillRatePercent: minKillRate,
	}
	if check == nil {
		result.ThresholdError = fmt.Errorf("%w: no mutation kill report data", ErrMutationKillsFailed)
		return result
	}
	result.Denominator = check.TotalKilled + check.TotalLived
	result.MaxLivedAllowed = maxLivedAllowedForMinKillRate(check.TotalKilled, minKillRate)
	result.LivedOverBudget = check.TotalLived - result.MaxLivedAllowed
	if result.LivedOverBudget < 0 {
		result.LivedOverBudget = 0
	}
	result.ThresholdError = CheckMutationKillsRate(check, minKillRate)
	result.Passed = result.ThresholdError == nil
	return result
}

// MutationKills checks mutation kill rate against the threshold.
// minKillRate is the required kill rate percentage (0-100).
// A minKillRate of 0 disables the check (always passes).
func MutationKills(jsonData []byte, minKillRate int) (MutationKillsResult, error) {
	if minKillRate < 0 || minKillRate > killRatePercentScale {
		return MutationKillsResult{}, fmt.Errorf("%w: got %d", errInvalidMinKillRate, minKillRate)
	}

	root, err := parseGremlinsMutationRoot(jsonData)
	if err != nil {
		return MutationKillsResult{}, err
	}

	check, parseErr := buildMutationKillsCheckFromRoot(root)
	if parseErr != nil {
		return MutationKillsResult{}, parseErr
	}

	computeKillRate(check)

	return EvaluateMutationKills(check, minKillRate), nil
}
