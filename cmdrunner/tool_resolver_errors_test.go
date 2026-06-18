// Vision: Tool resolution failure modes: missing tools, bad specs, and subprocess errors from probe commands.
// Parser internals (parseGoListMOutput, parseModuleFromVersionOutput) are tested via white-box tests in
// tool_resolver_parse_internal_test.go.
package cmdrunner

import (
	"context"
	"errors"
	"testing"
)

func TestResolveToolCommand_BinaryModuleMetadataAbsent_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{
		ListStdout:    pkgGolangciLint + " " + versionLint + "\n",
		VersionStdout: "no mod line here\n",
	}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for missing module metadata, got nil")
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_GoListEmptyOutput_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{ListStdout: ""}
	resolver := mustTestResolver(t, lookPath, fakeRunnerForGoProbe(&cfg))
	binary, args, err := resolver.ResolveToolCommand(
		ctx,
		"golangci-lint",
		"github.com/golangci/golangci-lint@v1.50.0",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for go list empty output, got nil")
	}
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestResolveToolCommand_GoListUnexpectedFormat_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lookPath := func(string) (string, error) { return localPathGolangciLint, nil }
	cfg := goToolFakeConfig{ListStdout: pkgGolangciLint + "\n"}
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
	if binary != "" || args != nil {
		t.Fatalf("expected empty result on error, got binary=%q args=%v", binary, args)
	}
}

func TestNewToolResolver_NilRunner_UsesErrNilDependency(t *testing.T) {
	t.Parallel()
	lp := func(string) (string, error) { return "", nil }
	_, err := newToolResolver(nil, lp)
	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected errors.Is(err, ErrNilDependency), got: %v", err)
	}
}
