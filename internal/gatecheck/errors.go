// Vision: Tool-failure sentinels for re-export via gate/errors.go ([errors.Is] stable across packages).
// Harness wiring mistakes stay in internal/harness/errors.go instead of this file.
package gatecheck

import "errors"

var (
	// Step failure sentinels — re-exported from gate/errors.go for external consumers.
	ErrLintFailed         = errors.New("lint failed")
	ErrFormatFailed       = errors.New("format failed")
	ErrDeadcodeFailed     = errors.New("deadcode failed")
	ErrMarkdownLintFailed = errors.New("markdownlint failed")
	// ErrCompileFailed matches the compile step id (short name, like lint/vet) so
	// Verbose-display raw errors and silent-display diagnostics share one prefix.
	ErrCompileFailed               = errors.New("compile failed")
	ErrVetFailed                   = errors.New("vet failed")
	ErrTestFailed                  = errors.New("test failed")
	ErrDurationFailed              = errors.New("duration failed")
	ErrCoverageFailed              = errors.New("coverage failed")
	ErrCrapFailed                  = errors.New("crap failed")
	ErrQualityScopeInventoryFailed = errors.New("qualityscopeinventory failed")
	ErrMutationSitesFailed         = errors.New("mutationsites failed")
	ErrMutationKillsFailed         = errors.New("mutationkills failed")
	ErrMutationCoverageFailed      = errors.New("mutationcoverage failed")

	// ErrAllPackagesExcluded is returned when filtering removes every package from
	// duration, mutation, or coverage package lists, indicating the exclude configuration is too aggressive.
	ErrAllPackagesExcluded = errors.New("all packages excluded by exclude configuration")

	// ErrQualityScopeSourceInventoryRequired is returned when a quality-scope command
	// projection needs the root-relative .go source inventory but none was supplied.
	ErrQualityScopeSourceInventoryRequired = errors.New("quality scope source inventory required")

	// ErrInvalidTestFilePattern is returned when a quality-scope test file glob is malformed.
	ErrInvalidTestFilePattern = errors.New("invalid test file pattern")

	// Internal validation and data sentinels.
	errInvalidMaxSites           = errors.New("maxSites must be > 0")
	errInvalidMinKillRate        = errors.New("minKillRate must be between 0 and 100")
	errInvalidMaxDurationSeconds = errors.New("duration maxSeconds must be positive")
	errNoTestCompletions         = errors.New("no test completion events in test JSON stream")
	errMissingStatus             = errors.New("missing or non-string status in mutation")
	errEmptyStatus               = errors.New("empty mutation status")
	errUnknownStatus             = errors.New("unknown mutation status")

	// Gremlins mutations.json structural parse failures (wrapped with detail).
	errGremlinsFilesNotArray         = errors.New(`gremlins top-level "files" must be a JSON array`)
	errGremlinsMutationsNotArray     = errors.New(`gremlins top-level "mutations" must be a JSON array`)
	errGremlinsFileMutationsNotArray = errors.New(`gremlins files[].mutations must be a JSON array`)
)
