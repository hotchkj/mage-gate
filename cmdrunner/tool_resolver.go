// Vision: Generic pinned-tool resolution abstraction for probing local binaries
// and deciding between local execution and "go run" fallback.
// Subprocess work goes through CommandRunner + Capture; PATH lookup uses lookPath only.
package cmdrunner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"regexp"
	"strings"
)

// ToolResolver picks a local binary when its module version matches toolSpec, else `go run` with that spec.
type ToolResolver interface {
	// ResolveToolCommand returns ("binary", nil, nil) on exact match, ("go", []string{"run", ...}, nil) for fallback,
	// or an error when version probing fails (callers must not treat that as fallback).
	ResolveToolCommand(ctx context.Context, toolName, toolSpec string, extraArgs []string) (string, []string, error)
}

type pathLookuper func(name string) (string, error)

// productionToolResolver implements ToolResolver using CommandRunner for all go subprocesses.
type productionToolResolver struct {
	runner   CommandRunner
	lookPath pathLookuper
}

const modulePathVersionFieldCount = 2

const toolResolverWorkDir = "."

// NewProductionToolResolver returns a ToolResolver that probes the local machine
// using exec.LookPath and "go version -m" / "go list -m" via [NewProductionRunner].
func NewProductionToolResolver() ToolResolver {
	resolver, _ := newToolResolver(NewProductionRunner(), exec.LookPath)
	return resolver
}

// Production callers use [NewProductionToolResolver]; tests that need deterministic
// resolver behavior should inject their own [ToolResolver].
func newToolResolver(runner CommandRunner, lookPath pathLookuper) (ToolResolver, error) {
	if runner == nil {
		return nil, fmt.Errorf("%w: CommandRunner cannot be nil", ErrNilDependency)
	}
	if lookPath == nil {
		return nil, fmt.Errorf("%w: lookPath cannot be nil", ErrNilDependency)
	}
	return &productionToolResolver{runner: runner, lookPath: lookPath}, nil
}

// ResolveToolCommand validates toolSpec, probes the local binary, and either returns it, `go run`, or a probe error.
func (r *productionToolResolver) ResolveToolCommand(
	ctx context.Context,
	toolName, toolSpec string,
	extraArgs []string,
) (binary string, args []string, err error) {
	if specErr := ValidateToolSpec(toolSpec); specErr != nil {
		return "", nil, specErr
	}

	localPath, err := r.lookPath(toolName)
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, fs.ErrNotExist) {
			return "", nil, fmt.Errorf("look up %s: %w", toolName, err)
		}
		return "go", buildGoRunArgs(toolSpec, extraArgs), nil
	}

	match, probeErr := probeBinaryVersionWithProber(ctx, r.runner, localPath, toolSpec)
	if probeErr != nil {
		return "", nil, fmt.Errorf("probe %s version: %w", toolName, probeErr)
	}

	if match {
		return localPath, extraArgs, nil
	}

	return "go", buildGoRunArgs(toolSpec, extraArgs), nil
}

func probeBinaryVersionWithProber(
	ctx context.Context,
	runner CommandRunner,
	binaryPath, toolSpec string,
) (bool, error) {
	parts := strings.Split(toolSpec, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, fmt.Errorf("%w: %q (expected 'package@version')", ErrToolSpecInvalid, toolSpec)
	}
	configPackagePath := parts[0]
	configVersion := parts[1]

	expectedModPath, resolveErr := resolveModulePathViaGoListWithProber(ctx, runner, configPackagePath, configVersion)
	if resolveErr != nil {
		return false, resolveErr
	}

	probeModPath, probeVersion, probeErr := goVersionModuleMetadata(ctx, runner, binaryPath)
	if probeErr != nil {
		return false, probeErr
	}

	return probeModPath == expectedModPath && probeVersion == configVersion, nil
}

func resolveModulePathViaGoListWithProber(
	ctx context.Context,
	runner CommandRunner,
	packagePath, version string,
) (string, error) {
	candidate := packagePath
	var lastErr error
	for {
		modulePath, moduleVersion, err := goListModuleVersions(ctx, runner, candidate+"@"+version)
		if err == nil {
			if moduleVersion != version {
				return "", fmt.Errorf(
					"%w: %s@%s -> %q",
					errGoListResolvedVersion,
					candidate,
					version,
					moduleVersion,
				)
			}
			return modulePath, nil
		}
		lastErr = err

		cut := strings.LastIndex(candidate, "/")
		if cut < 0 {
			break
		}
		candidate = candidate[:cut]
	}

	return "", fmt.Errorf(
		"resolve module for %s@%s: %w",
		packagePath,
		version,
		lastErr,
	)
}

func goListModuleVersions(
	ctx context.Context,
	runner CommandRunner,
	spec string,
) (modulePath, moduleVersion string, err error) {
	res, runErr := Capture(ctx, runner, toolResolverWorkDir, "go", "list", "-m", "-f", "{{.Path}} {{.Version}}", spec)
	out := strings.TrimSpace(res.Stdout)
	if runErr != nil {
		return "", "", fmt.Errorf("go list %s failed: %w (output: %s)", spec, runErr, out+res.Stderr)
	}
	return parseGoListMOutput(out, spec)
}

func parseGoListMOutput(line, spec string) (modulePath, moduleVersion string, err error) {
	if line == "" {
		return "", "", fmt.Errorf("%w: %s", errGoListEmptyOutput, spec)
	}
	fields := strings.Fields(line)
	if len(fields) < modulePathVersionFieldCount {
		return "", "", fmt.Errorf("%w: %s -> %q", errGoListUnexpectedFormat, spec, line)
	}
	return fields[0], fields[1], nil
}

func goVersionModuleMetadata(
	ctx context.Context,
	runner CommandRunner,
	binaryPath string,
) (modulePath, moduleVersion string, err error) {
	res, runErr := Capture(ctx, runner, toolResolverWorkDir, "go", "version", "-m", binaryPath)
	combined := res.Stdout + res.Stderr
	if runErr != nil {
		return "", "", fmt.Errorf("go version -m failed: %w (output: %s)", runErr, combined)
	}
	return parseModuleFromVersionOutput(res.Stdout)
}

// buildGoRunArgs constructs the full argument list for "go run <spec> <extraArgs>".
func buildGoRunArgs(toolSpec string, extraArgs []string) []string {
	args := []string{"run", toolSpec}
	args = append(args, extraArgs...)
	return args
}

func parseModuleFromVersionOutput(output string) (modulePath, moduleVersion string, err error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "mod" {
			modPath := fields[1]
			modVersion := fields[2]
			if modPath != "" && modVersion != "" {
				return modPath, modVersion, nil
			}
		}
	}

	return "", "", errBinaryModuleMetadataAbsent
}

// toolSpecRegex is a regexp pattern for validating tool specs at the boundary.
// Matches "package@version" format with non-empty parts.
var toolSpecRegex = regexp.MustCompile(`^[a-zA-Z0-9./_-]+@[a-zA-Z0-9.\-\+]+$`)

var (
	errGoListResolvedVersion      = errors.New("go list resolved unexpected version")
	errGoListEmptyOutput          = errors.New("go list returned empty output")
	errGoListUnexpectedFormat     = errors.New("go list returned unexpected format")
	errBinaryModuleMetadataAbsent = errors.New("module metadata not found in binary build info")

	// ErrToolSpecEmpty indicates that the tool spec is empty.
	ErrToolSpecEmpty = errors.New("tool spec cannot be empty")

	// ErrToolSpecInvalid indicates that the tool spec format is invalid (expected 'package@version').
	ErrToolSpecInvalid = errors.New("invalid tool spec format")
)

// ValidateToolSpec checks whether spec is in the format "package@version".
func ValidateToolSpec(spec string) error {
	if spec == "" {
		return ErrToolSpecEmpty
	}
	if !toolSpecRegex.MatchString(spec) {
		return fmt.Errorf("%w: %q (expected 'package@version')", ErrToolSpecInvalid, spec)
	}
	return nil
}
