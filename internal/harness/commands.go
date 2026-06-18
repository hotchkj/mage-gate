// Vision: Single runCommand entrypoint so every harness step shares argv/env/cwd rules and capture behavior.
package harness

import (
	"context"
	"fmt"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

// runCommand wraps [cmdrunner.Capture]; failed runs still return partial stdout/stderr in [cmdrunner.CommandResult].
func (h *StepRunner) runCommand(
	ctx context.Context, dir, name string, args ...string,
) (cmdrunner.CommandResult, error) {
	result, err := cmdrunner.Capture(ctx, h.runner, dir, name, args...)
	if err != nil {
		return result, fmt.Errorf("%s: %w; stdout=%s; stderr=%s",
			name, err, result.Stdout, result.Stderr)
	}
	return result, nil
}
