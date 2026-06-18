// Vision: Vet step: assemble `go vet` for the package scope, honor extra vet flags, surface stderr-shaped failures.
package harness

import (
	"context"
	"fmt"
)

// StepVet runs go vet with the given extra arguments and harness package scope.
func (h *StepRunner) StepVet(ctx context.Context, vetArgs []string) error {
	pkgs := resolvePackages(h.packages)
	args := h.buildVetArgs(pkgs, vetArgs)
	if _, err := h.runCommand(ctx, h.root, "go", args...); err != nil {
		return fmt.Errorf("%w: go vet: %w", ErrVetFailed, err)
	}
	return nil
}

func (h *StepRunner) buildVetArgs(pkgs string, vetArgs []string) []string {
	args := []string{"vet"}
	args = append(args, vetArgs...)
	args = append(args, pkgs)
	return args
}
