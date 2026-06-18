// Vision: Lint step: resolve pinned golangci-lint, build if needed, run with repo config, return full stderr on fail.
package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
)

// StepLint runs golangci-lint: customGCLPath forces local custom build; else local binary or `go run`.
func (h *StepRunner) StepLint(
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

	if err := h.runLinter(ctx, lintConfigPath, customGCLPath, lintToolSpec, lintArgs, "run", ErrLintFailed); err != nil {
		return fmt.Errorf("%w: run unit lint: %w", ErrLintFailed, err)
	}
	return nil
}

func (h *StepRunner) ensureLintArtifactDir() error {
	if err := h.fileOps.MkdirAll(h.artifacts.Dir(), artifactDirPerm); err != nil {
		return fmt.Errorf("%w: create artifact dir: %w", ErrLintFailed, err)
	}
	return nil
}

func (h *StepRunner) buildLintIfNeeded(ctx context.Context, customGCLPath, customLintSpec string) error {
	if customGCLPath == "" {
		return nil
	}
	if customLintSpec == "" {
		return fmt.Errorf("%w: CustomLintToolSpec is required when CustomGCL is set", ErrLintFailed)
	}
	if err := h.buildCustomLinter(ctx, customGCLPath, customLintSpec); err != nil {
		return fmt.Errorf("%w: build custom golangci-lint: %w", ErrLintFailed, err)
	}
	return nil
}

func (h *StepRunner) buildCustomLinter(ctx context.Context, customGCLPath, customLintSpec string) error {
	workDir := h.root
	if customGCLPath != "" {
		// Caller-provided host/config path—not a canonical logical artifact layout path.
		workDir = filepath.Dir(customGCLPath)
	}
	destHostPath, err := h.artifacts.HostPath()
	if err != nil {
		return fmt.Errorf("%w: custom linter destination host path: %w", ErrLintFailed, err)
	}
	_, err = h.runCommand(
		ctx,
		workDir,
		"go",
		"run",
		customLintSpec,
		"custom",
		"--destination",
		destHostPath,
		"--name",
		customLintBinary,
	)
	return err
}

func (h *StepRunner) runLinter(
	ctx context.Context,
	lintConfigPath, customGCLPath, lintToolSpec string,
	lintArgs []string,
	subcommand string,
	failSentinel error,
) error {
	if lintConfigPath == "" {
		return fmt.Errorf("%w: LintConfigPath is required (empty means misconfigured harness)", failSentinel)
	}
	if customGCLPath == "" && h.toolResolver == nil {
		return fmt.Errorf("%w: ToolResolver is required", failSentinel)
	}

	binary, args, err := h.resolveLinterCommand(
		ctx, subcommand, lintConfigPath, customGCLPath, lintToolSpec, lintArgs,
	)
	if err != nil {
		return fmt.Errorf("%w: resolve linter command: %w", failSentinel, err)
	}
	_, runErr := h.runCommand(ctx, h.root, binary, args...)
	return runErr
}

func customLintExeLogicalName() string {
	if runtime.GOOS == "windows" {
		return customLintBinary + ".exe"
	}
	return customLintBinary
}

func (h *StepRunner) resolveLinterCommand(
	ctx context.Context,
	subcommand, lintConfigPath, customGCLPath, lintToolSpec string,
	lintArgs []string,
) (binary string, args []string, err error) {
	pkgs := resolvePackages(h.packages)
	if customGCLPath != "" {
		binary, err = h.artifacts.CommandPath(customLintExeLogicalName())
		if err != nil {
			return "", nil, err
		}
		out := []string{subcommand, "-c", lintConfigPath, pkgs}
		out = append(out, lintArgs...)
		return binary, out, nil
	}

	extraArgs := []string{subcommand, "-c", lintConfigPath, pkgs}
	extraArgs = append(extraArgs, lintArgs...)
	binary, args, err = h.toolResolver.ResolveToolCommand(ctx, "golangci-lint", lintToolSpec, extraArgs)
	if err != nil {
		return "", nil, err
	}
	return binary, args, nil
}
