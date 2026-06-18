// Vision: Functional options for StepRunner—compose runners, stores, roots, and quality scopes without globals.
package harness

import (
	"github.com/hotchkj/mage-gate/cmdrunner"
)

type StepRunnerOption func(*StepRunner)

func WithToolResolver(resolver cmdrunner.ToolResolver) StepRunnerOption {
	return func(h *StepRunner) {
		h.toolResolver = resolver
	}
}
