// Vision: Format step: same linter resolution as lint; subcommand fmt applies configured formatters.
package harness

import (
	"context"
	"fmt"
)

// StepFormat runs golangci-lint fmt: customGCLPath forces local custom build; else local binary or `go run`.
func (h *StepRunner) StepFormat(
	ctx context.Context,
	lintConfigPath, customGCLPath, customLintSpec, lintToolSpec string,
	lintArgs []string,
) error {
	if err := h.ensureLintArtifactDir(); err != nil {
		return err
	}

	if err := h.buildLintIfNeeded(ctx, customGCLPath, customLintSpec); err != nil {
		return err
	}

	if err := h.runLinter(ctx, lintConfigPath, customGCLPath, lintToolSpec, lintArgs, "fmt", ErrFormatFailed); err != nil {
		return fmt.Errorf("%w: run format: %w", ErrFormatFailed, err)
	}
	return nil
}
