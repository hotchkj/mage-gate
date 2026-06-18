// Vision: Markdown lint step: resolve pinned gomarklint, run from repo root with consumer argv only.
package harness

import (
	"context"
	"fmt"
)

// StepMarkdownLint runs gomarklint from h.root with consumer-supplied args only (no package scope).
func (h *StepRunner) StepMarkdownLint(ctx context.Context, toolSpec string, args []string) error {
	if h.toolResolver == nil {
		return fmt.Errorf("%w: ToolResolver is required", ErrMarkdownLintFailed)
	}

	extraArgs := append([]string{}, args...)
	binary, resolvedArgs, err := h.toolResolver.ResolveToolCommand(ctx, "gomarklint", toolSpec, extraArgs)
	if err != nil {
		return fmt.Errorf("%w: resolve gomarklint command: %w", ErrMarkdownLintFailed, err)
	}

	if _, err := h.runCommand(ctx, h.root, binary, resolvedArgs...); err != nil {
		return fmt.Errorf("%w: gomarklint command: %w", ErrMarkdownLintFailed, err)
	}
	return nil
}
