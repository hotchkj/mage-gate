// Vision: Harness-only sentinels for wiring mistakes (deps, artifacts, roots) distinct from gatecheck tool failures.
package harness

import (
	"errors"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// Step failure sentinels — aliased from gatecheck so they are re-exportable from gate/errors.go.
var (
	ErrLintFailed          = gatecheck.ErrLintFailed
	ErrFormatFailed        = gatecheck.ErrFormatFailed
	ErrDeadcodeFailed      = gatecheck.ErrDeadcodeFailed
	ErrMarkdownLintFailed  = gatecheck.ErrMarkdownLintFailed
	ErrCompileFailed       = gatecheck.ErrCompileFailed
	ErrVetFailed           = gatecheck.ErrVetFailed
	ErrTestFailed          = gatecheck.ErrTestFailed
	ErrDurationFailed      = gatecheck.ErrDurationFailed
	ErrCoverageFailed      = gatecheck.ErrCoverageFailed
	ErrCrapFailed          = gatecheck.ErrCrapFailed
	ErrMutationSitesFailed = gatecheck.ErrMutationSitesFailed
	ErrMutationKillsFailed = gatecheck.ErrMutationKillsFailed
)

// Internal harness sentinels — not re-exported; these are infra errors, not step failures.
var (
	ErrArtifactStoreRequired = errors.New("artifact store required")
	ErrArtifactKeyMissing    = errors.New("artifact key missing")
	ErrRootRequired          = errors.New("root required")
	ErrDepsRequired          = errors.New("dependencies required")
	ErrPathTraversal         = fileopspath.ErrPathTraversal
	// errGoListMutationLineInvalid is wrapped when a go list -f line has an unexpected field count.
	errGoListMutationLineInvalid = errors.New("go list: expected exactly 6 tab fields in mutation list line")
)

// CoverageFailure wraps coverage threshold failures with stable unwrapping.
type CoverageFailure struct {
	result gatecheck.CoverageResult
}

func (e *CoverageFailure) Error() string {
	if e.result.ThresholdError != nil {
		return e.result.ThresholdError.Error()
	}
	return ErrCoverageFailed.Error()
}

// Unwrap returns ErrCoverageFailed for errors.Is stability.
func (e *CoverageFailure) Unwrap() error {
	if e.result.ThresholdError != nil {
		return e.result.ThresholdError
	}
	return ErrCoverageFailed
}

// Result returns the coverage result fields for diagnostics.
func (e *CoverageFailure) Result() gatecheck.CoverageResult {
	return e.result
}
