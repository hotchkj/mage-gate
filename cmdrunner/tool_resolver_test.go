// Vision: ToolResolver: local binary vs go-run fallback, PATH edge cases, and resolver errors under a mocked runner.
package cmdrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"
)

var (
	errNotFoundInPath = fmt.Errorf("not found in PATH: %w", fs.ErrNotExist)
	errUnexpectedPath = errors.New("unexpected path")
	errProbeFailure   = errors.New("go version -m failed: exit code 1")
	errGoListFailure  = errors.New("go list failed: cannot find module")
	errUnexpectedCmd  = errors.New("unexpected command")
)

const (
	localPathGolangciLint = "/usr/local/bin/golangci-lint"
	pkgGolangciLint       = "github.com/golangci/golangci-lint"
	versionLint           = "v1.50.0"
)

// goToolFakeConfig drives fake CommandRunner responses for resolver probes.
type goToolFakeConfig struct {
	ListStdout     string
	VersionStdout  string
	ListErr        error
	VersionErr     error
	WantVersionArg string
}

type resolverProbeFunc func(ctx context.Context, cmd Command, stdout, stderr io.Writer) error

type resolverProbeRunner struct {
	responses map[string]resolverProbeFunc
}

func newResolverProbeRunner(responses map[string]resolverProbeFunc) *resolverProbeRunner {
	return &resolverProbeRunner{responses: responses}
}

func (r *resolverProbeRunner) Run(
	ctx context.Context,
	dir string,
	stdout, stderr io.Writer,
	name string,
	args ...string,
) error {
	cmd := NewCommand(dir, name, args...)
	if cmd.Arg(0) != "" && cmd.Arg(1) != "" {
		key := name + " " + cmd.Arg(0) + " " + cmd.Arg(1)
		if response := r.responses[key]; response != nil {
			return response(ctx, cmd, stdout, stderr)
		}
	}
	return fmt.Errorf("%w: %s %v", errUnexpectedCmd, name, args)
}

func fakeRunnerForGoProbe(cfg *goToolFakeConfig) *resolverProbeRunner {
	responses := map[string]resolverProbeFunc{
		"go list -m": func(_ context.Context, _ Command, stdout, _ io.Writer) error {
			if cfg.ListErr != nil {
				return cfg.ListErr
			}
			_, err := io.WriteString(stdout, cfg.ListStdout)
			return err
		},
	}
	if cfg.VersionStdout != "" || cfg.VersionErr != nil || cfg.WantVersionArg != "" {
		responses["go version -m"] = func(
			_ context.Context,
			cmd Command,
			stdout, _ io.Writer,
		) error {
			if cfg.WantVersionArg != "" && cmd.Arg(2) != cfg.WantVersionArg {
				return errUnexpectedPath
			}
			if cfg.VersionErr != nil {
				return cfg.VersionErr
			}
			_, err := io.WriteString(stdout, cfg.VersionStdout)
			return err
		}
	}
	return newResolverProbeRunner(responses)
}

func mustTestResolver(
	tb testing.TB,
	lookPath pathLookuper,
	runner CommandRunner,
) ToolResolver {
	tb.Helper()
	resolver, err := newToolResolver(runner, lookPath)
	if err != nil {
		tb.Fatal(err)
	}
	return resolver
}

func TestValidateToolSpec_EmptySpec_ReturnsErrToolSpecEmpty(t *testing.T) {
	t.Parallel()
	err := ValidateToolSpec("")
	if err == nil {
		t.Fatal("expected error for empty spec, got nil")
	}
	if !errors.Is(err, ErrToolSpecEmpty) {
		t.Fatalf("expected ErrToolSpecEmpty, got %v", err)
	}
}

func TestValidateToolSpec_ValidSpecs_ReturnsNil(t *testing.T) {
	t.Parallel()
	validSpecs := []string{
		"github.com/golangci/golangci-lint@v1.0.0",
		"golang.org/x/tools@v1.2.3",
		"github.com/user/repo@v0.0.1-rc1",
		"pkg.local/tool@v1.0.0+build",
	}
	for _, spec := range validSpecs {
		t.Run(spec, func(t *testing.T) {
			err := ValidateToolSpec(spec)
			if err != nil {
				t.Fatalf("expected nil for valid spec %q, got %v", spec, err)
			}
		})
	}
}

func TestValidateToolSpec_InvalidFormats_ReturnsErrToolSpecInvalid(t *testing.T) {
	t.Parallel()
	invalidSpecs := []string{
		"github.com/golangci/golangci-lint",
		"@v1.0.0",
		"github.com/golangci/golangci-lint@",
		"github.com/golangci/golangci-lint v1.0.0",
		"github com/golangci/golangci-lint@v1.0.0",
	}
	for _, spec := range invalidSpecs {
		t.Run(spec, func(t *testing.T) {
			err := ValidateToolSpec(spec)
			if err == nil {
				t.Fatalf("expected error for invalid spec %q, got nil", spec)
			}
			if !errors.Is(err, ErrToolSpecInvalid) {
				t.Fatalf("expected ErrToolSpecInvalid, got %v", err)
			}
		})
	}
}

func TestNewToolResolver_NilLookPath_ReturnsError(t *testing.T) {
	t.Parallel()
	inner := newResolverProbeRunner(nil)
	resolver, err := newToolResolver(inner, nil)
	if err == nil {
		t.Fatal("expected error for nil lookPath, got nil")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
	if resolver != nil {
		t.Fatalf("expected nil resolver on error, got %v", resolver)
	}
}

func TestNewToolResolver_ValidDeps_ReturnsResolver(t *testing.T) {
	t.Parallel()
	inner := newResolverProbeRunner(nil)
	lp := func(string) (string, error) { return "", nil }
	resolver, err := newToolResolver(inner, lp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver, got nil")
	}
}

func TestResolveToolCommand_EmptyToolSpec_ReturnsErrToolSpecEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(name string) (string, error) {
		t.Fatalf("LookPath should not be called, got %q", name)
		return "", nil
	}
	resolver := mustTestResolver(t, lookPath, newResolverProbeRunner(nil))
	binary, args, err := resolver.ResolveToolCommand(ctx, "some-tool", "", nil)
	if err == nil {
		t.Fatal("expected error for empty toolSpec, got nil")
	}
	if !errors.Is(err, ErrToolSpecEmpty) {
		t.Fatalf("expected ErrToolSpecEmpty, got %v", err)
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_InvalidToolSpecFormat_ReturnsErrToolSpecInvalid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(name string) (string, error) {
		t.Fatalf("LookPath should not be called, got %q", name)
		return "", nil
	}
	resolver := mustTestResolver(t, lookPath, newResolverProbeRunner(nil))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"some-tool",
		"github.com/golangci/golangci-lint",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for invalid toolSpec, got nil")
	}
	if !errors.Is(err, ErrToolSpecInvalid) {
		t.Fatalf("expected ErrToolSpecInvalid, got %v", err)
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_BinaryNotFound_FallbackToGoRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(name string) (string, error) {
		if name != "golangci-lint" {
			t.Errorf("expected LookPath(golangci-lint), got %q", name)
		}
		return "", errNotFoundInPath
	}
	resolver := mustTestResolver(t, lookPath, newResolverProbeRunner(nil))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		[]string{"--config", ".golangci.yml"},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != "go" {
		t.Fatalf("expected binary=go, got %q", binary)
	}
	expectedArgs := []string{
		"run", "github.com/golangci/golangci-lint@v1.50.0", "--config", ".golangci.yml",
	}
	if len(args) != len(expectedArgs) {
		t.Fatalf("expected args len %d, got %d", len(expectedArgs), len(args))
	}
	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("args[%d]: expected %q, got %q", i, expected, args[i])
		}
	}
}

func TestResolveToolCommand_LookPathFailure_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return "", errUnexpectedPath }
	resolver := mustTestResolver(t, lookPath, newResolverProbeRunner(nil))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		[]string{"--config", ".golangci.yml"},
	)
	if err == nil {
		t.Fatal("expected lookup error")
	}
	if !errors.Is(err, errUnexpectedPath) {
		t.Fatalf("expected wrapped lookup error, got %v", err)
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_BinaryFound_VersionMatches_ReturnLocalPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{
		ListStdout:    pkgGolangciLint + " " + versionLint + "\n",
		VersionStdout: "\tmod " + pkgGolangciLint + " " + versionLint + "\n",
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		[]string{"--config", ".golangci.yml"},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != localPathGolangciLint {
		t.Fatalf("expected binary=%q, got %q", localPathGolangciLint, binary)
	}
	expectedArgs := []string{"--config", ".golangci.yml"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("expected args len %d, got %d", len(expectedArgs), len(args))
	}
	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("args[%d]: expected %q, got %q", i, expected, args[i])
		}
	}
}

func TestResolveToolCommand_BinaryFound_VersionMismatch_FallbackToGoRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{
		ListStdout:    pkgGolangciLint + " " + versionLint + "\n",
		VersionStdout: "\tmod " + pkgGolangciLint + " v1.49.0\n",
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		[]string{"--config", ".golangci.yml"},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != "go" {
		t.Fatalf("expected binary=go, got %q", binary)
	}
	expectedArgs := []string{
		"run", "github.com/golangci/golangci-lint@v1.50.0", "--config", ".golangci.yml",
	}
	if len(args) != len(expectedArgs) {
		t.Fatalf("expected args len %d, got %d", len(expectedArgs), len(args))
	}
	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("args[%d]: expected %q, got %q", i, expected, args[i])
		}
	}
}

func TestResolveToolCommand_VersionProbeFails_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{
		ListStdout: pkgGolangciLint + " " + versionLint + "\n",
		VersionErr: errProbeFailure,
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errProbeFailure) {
		t.Errorf("expected errProbeFailure in error chain, got %v", err)
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_ModuleResolutionFails_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{ListErr: errGoListFailure}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errGoListFailure) {
		t.Errorf("expected errGoListFailure in error chain, got %v", err)
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_ModuleMismatch_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{ListStdout: pkgGolangciLint + " v1.51.0\n"}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for version mismatch from go list, got nil")
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_ExtraArgsPreservedOnGoRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return "", errNotFoundInPath }
	resolver := mustTestResolver(t, lookPath, newResolverProbeRunner(nil))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"some-tool",
		"pkg/tool@v1.0.0",
		[]string{"arg1", "arg2", "arg3"},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != "go" {
		t.Fatalf("expected binary=go, got %q", binary)
	}
	expectedArgs := []string{"run", "pkg/tool@v1.0.0", "arg1", "arg2", "arg3"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("expected args len %d, got %d", len(expectedArgs), len(args))
	}
	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("args[%d]: expected %q, got %q", i, expected, args[i])
		}
	}
}

func TestResolveToolCommand_EmptyExtraArgs_NoExtraArgs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	localPath := "/usr/bin/tool"
	lookPath := func(string) (string, error) { return localPath, nil }
	cfg := goToolFakeConfig{
		ListStdout:    "pkg/tool v1.0.0\n",
		VersionStdout: "\tmod pkg/tool v1.0.0\n",
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(ctx, "tool", "pkg/tool@v1.0.0", nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != localPath {
		t.Fatalf("expected binary=%q, got %q", localPath, binary)
	}
	if len(args) != 0 {
		t.Fatalf("expected empty args, got %v", args)
	}
}

func TestResolveToolCommand_BinaryPathWithSpace_Match(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	localPath := "/Program Files/bin/golangci-lint"
	lookPath := func(string) (string, error) { return localPath, nil }
	cfg := goToolFakeConfig{
		ListStdout:     pkgGolangciLint + " " + versionLint + "\n",
		VersionStdout:  "\tmod " + pkgGolangciLint + " " + versionLint + "\n",
		WantVersionArg: localPath,
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, _, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if binary != localPath {
		t.Fatalf("expected binary=%q, got %q", localPath, binary)
	}
}
