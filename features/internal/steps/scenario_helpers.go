// Vision: Shared assertion helpers for Then/And steps—compare diagnostics, artifacts, and command transcripts.
package steps

import (
	"errors"
	"fmt"

	qg "github.com/hotchkj/mage-gate/gate"
)

var knownErrors = map[string]error{
	"ErrLintFailed":             qg.ErrLintFailed,
	"ErrFormatFailed":           qg.ErrFormatFailed,
	"ErrDeadcodeFailed":         qg.ErrDeadcodeFailed,
	"ErrMarkdownLintFailed":     qg.ErrMarkdownLintFailed,
	"ErrCompileFailed":          qg.ErrCompileFailed,
	"ErrVetFailed":              qg.ErrVetFailed,
	"ErrTestFailed":             qg.ErrTestFailed,
	"ErrDurationFailed":         qg.ErrDurationFailed,
	"ErrCoverageFailed":         qg.ErrCoverageFailed,
	"ErrCrapFailed":             qg.ErrCrapFailed,
	"ErrMutationSitesFailed":    qg.ErrMutationSitesFailed,
	"ErrMutationKillsFailed":    qg.ErrMutationKillsFailed,
	"ErrAllPackagesExcluded":    qg.ErrAllPackagesExcluded,
	"ErrInvalidOption":          qg.ErrInvalidOption,
	"ErrMutationCoverageFailed": qg.ErrMutationCoverageFailed,
}

func (s *scenarioState) assertErrorIs(errName string) error {
	if s.runErr == nil {
		return fmt.Errorf("%w: %q", errExpectedErrorButGotNil, errName)
	}
	expected, ok := knownErrors[errName]
	if !ok {
		return fmt.Errorf("%w: %q", errUnknownErrorType, errName)
	}
	if !errors.Is(s.runErr, expected) {
		return fmt.Errorf("expected %s but got %w", errName, s.runErr)
	}
	return nil
}
