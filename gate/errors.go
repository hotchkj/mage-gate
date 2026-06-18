// Vision: Exported error taxonomy — details in the section below (validation, diagnostics, CI sentinels, artifacts).
//
// Error taxonomy for consumers:
//
//   - ValidationError (.Step()): option wiring before tools (often wraps ErrInvalidOption / ErrLintConfigRequired).
//   - ErrQualityScopeEmpty, ErrMissingValue: raw pre-run input problems, not wrapped as ValidationError.
//   - DiagnosticError (.Name()): silent-display tool failure (ERROR/Fix/Hint), wrapping
//     step sentinels such as ErrLintFailed.
//   - Verbose display: step sentinels stay raw for errors.Is; no DiagnosticError wrapper.
//   - ErrNilDependency: nil CommandRunner, FileOps, ToolResolver, or *ArtifactStore.
//   - Artifact sentinels: ErrNoArtifactForStep, ErrArtifactNotFound, ErrArtifactSealed, ErrArtifactNotSealed.
//   - Other returned errors: harness/infra bugs—plain Go errors without a stable sentinel.
//
// Use errors.Is / errors.As on gate types for step wiring.
package gate

import (
	"errors"
	"fmt"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// Configuration and wiring errors.
var (
	ErrMissingValue                  = errors.New("required value not provided")
	ErrInvalidOption                 = errors.New("invalid option")
	ErrLintConfigRequired            = errors.New("lint config path is required")
	ErrNilDependency                 = errors.New("nil dependency")
	ErrInvalidOutputMode             = errors.New("invalid output mode")
	ErrQualityScopeEmpty             = errors.New("quality scope packages must not be empty")
	ErrPackageScopeEmpty             = errors.New("package scope pattern must not be empty")
	ErrQualityScopeInventoryInvalid  = errors.New("quality scope inventory token is invalid")
	ErrQualityScopeInventoryMismatch = errors.New("quality scope inventory does not match consumer")
	// ErrCoveredTestRequired: no [CoveredTestOutput] at all; incomplete tokens → [ErrMissingValue]. Use [errors.Is].
	ErrCoveredTestRequired = errors.New(
		"coverage requires CoveredTest output; call gate.CoveredTest before Coverage",
	)
	// ErrCoverpkgRequired is an alias of [ErrCoveredTestRequired] for older callers matching errors.Is.
	ErrCoverpkgRequired = ErrCoveredTestRequired
)

// Step failure sentinels — re-exported from internal/gatecheck so external consumers
// can errors.Is check for specific step failures.
var (
	ErrLintFailed                  = gatecheck.ErrLintFailed
	ErrFormatFailed                = gatecheck.ErrFormatFailed
	ErrDeadcodeFailed              = gatecheck.ErrDeadcodeFailed
	ErrMarkdownLintFailed          = gatecheck.ErrMarkdownLintFailed
	ErrCompileFailed               = gatecheck.ErrCompileFailed
	ErrVetFailed                   = gatecheck.ErrVetFailed
	ErrTestFailed                  = gatecheck.ErrTestFailed
	ErrDurationFailed              = gatecheck.ErrDurationFailed
	ErrCoverageFailed              = gatecheck.ErrCoverageFailed
	ErrCrapFailed                  = gatecheck.ErrCrapFailed
	ErrQualityScopeInventoryFailed = gatecheck.ErrQualityScopeInventoryFailed
	ErrMutationSitesFailed         = gatecheck.ErrMutationSitesFailed
	ErrMutationKillsFailed         = gatecheck.ErrMutationKillsFailed
	ErrMutationCoverageFailed      = gatecheck.ErrMutationCoverageFailed
	ErrAllPackagesExcluded         = gatecheck.ErrAllPackagesExcluded
)

// DiagnosticError re-exports cmdrunner.DiagnosticError for single-import consumer ergonomics.
type DiagnosticError = cmdrunner.DiagnosticError

// ValidationError provides detailed error context for validation failures.
// It wraps a sentinel (e.g. ErrInvalidOption) so errors.Is recognizes the cause.
type ValidationError struct {
	step    string
	message string
	cause   error
}

func (e *ValidationError) Error() string {
	if e.step != "" {
		return fmt.Sprintf("step %q: %s", e.step, e.message)
	}
	return e.message
}

func (e *ValidationError) Unwrap() error { return e.cause }

func (e *ValidationError) Step() string    { return e.step }
func (e *ValidationError) Message() string { return e.message }

func newValidationError(step, message string, cause error) error {
	return &ValidationError{
		step:    step,
		message: message,
		cause:   cause,
	}
}
