// Vision: LintToolchain construction validates lint inputs before step dispatch.
package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestNewLintToolchainRequiresConfig(t *testing.T) {
	_, err := NewLintToolchain(
		LintConfigValue{},
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err == nil {
		t.Fatal("expected error for missing LintConfig")
	}
	if !errors.Is(err, ErrLintConfigRequired) {
		t.Fatalf("expected ErrLintConfigRequired, got %v", err)
	}
}

func TestNewLintToolchainRequiresToolSpec(t *testing.T) {
	_, err := NewLintToolchain(LintConfig(".golangci.yml"), LintToolSpec(""))
	if err == nil {
		t.Fatal("expected error for missing LintToolSpec")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestNewLintToolchainRejectsCustomGCLWithoutCustomLintToolSpec(t *testing.T) {
	t.Parallel()
	_, err := NewLintToolchain(
		LintConfig(".golangci.yml"),
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
		CustomGCL(".custom-gcl.yml"),
	)
	if err == nil {
		t.Fatal("expected error when CustomGCL is configured without CustomLintToolSpec")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestLintRejectsZeroValueToolchain(t *testing.T) {
	pkgScope, err := NewPackageScope("./...")
	if err != nil {
		t.Fatalf("NewPackageScope() failed: %v", err)
	}
	err = Lint(
		context.Background(),
		noopGoFakeRunner(),
		gatetest.NewFakeToolResolver(),
		gatetest.NewMemoryFileOps(),
		".",
		pkgScope,
		LintToolchain{},
	)
	if err == nil {
		t.Fatal("expected error for zero-value LintToolchain")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}
