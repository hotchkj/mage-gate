// Vision: ToolSpec and small option types—split from validation_test.go for file-length limits.
package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestCrapRequiresMissingGocycloToolSpec(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	covOut := CoverageOutput{stepID: "c", qualityScope: scope}
	err = Crap(
		context.Background(), noopGoFakeRunner(), nil, store, mem, ".", covOut,
		QualityScopeInventoryOutput{}, MaxScore(8), GocycloToolValue{},
	)
	if err == nil {
		t.Fatal("expected error when GocycloToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestCrapRejectsMalformedGocycloToolSpec(t *testing.T) {
	err := validateGocycloToolSpec(GocycloToolSpec("not-a-valid-spec"))
	if err == nil {
		t.Fatal("expected malformed gocyclo tool spec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationRunnerScanRequiresMissingGremlinsToolSpec(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(noopGoFakeRunner(), resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	_, err = mr.Scan(
		context.Background(),
		".",
		scope,
		QualityScopeInventoryOutput{},
		GremlinsToolValue{},
	)
	if err == nil {
		t.Fatal("expected error when GremlinsToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsRequiresMissingGremlinsToolSpec(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		NewArtifactStore(),
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(0),
		GremlinsToolValue{},
	)
	if err == nil {
		t.Fatal("expected error when GremlinsToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationRejectsMalformedGremlinsToolSpec(t *testing.T) {
	err := validateGremlinsToolSpec("mutationsites", GremlinsToolSpec("not-a-valid-spec"))
	if err == nil {
		t.Fatal("expected malformed gremlins tool spec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestCustomLintToolSpecOption(t *testing.T) {
	var cfg lintConfig
	CustomLintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.99.0")(&cfg)
	if cfg.customLintSpec != "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.99.0" {
		t.Errorf("expected customLintSpec set, got %q", cfg.customLintSpec)
	}
}

func TestLintArgs_OptionPopulatesConfig(t *testing.T) {
	var cfg lintConfig
	LintArgs("--verbose", "--max-issues-per-linter=0")(&cfg)
	want := []string{"--verbose", "--max-issues-per-linter=0"}
	if len(cfg.lintArgs) != len(want) {
		t.Fatalf("lintArgs = %v, want %v", cfg.lintArgs, want)
	}
	for i := range want {
		if cfg.lintArgs[i] != want[i] {
			t.Fatalf("lintArgs[%d] = %q, want %q", i, cfg.lintArgs[i], want[i])
		}
	}
}

func TestCrapArgs_OptionPopulatesConfig(t *testing.T) {
	cfg := defaultCrapConfig()
	CrapArgs("-top", "10")(&cfg)
	if len(cfg.crapArgs) != 2 || cfg.crapArgs[0] != "-top" || cfg.crapArgs[1] != "10" {
		t.Fatalf("crapArgs = %v", cfg.crapArgs)
	}
}

func TestLintRequiresMissingToolSpec(t *testing.T) {
	_, err := NewLintToolchain(LintConfig(".golangci.yml"), LintToolSpec(""))
	if err == nil {
		t.Fatal("expected error when LintToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestLintRejectsCustomGCLWithoutCustomLintToolSpec(t *testing.T) {
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

func TestLintRejectsCustomLintToolSpecWithoutCustomGCL(t *testing.T) {
	t.Parallel()
	_, err := NewLintToolchain(
		LintConfig(".golangci.yml"),
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
		CustomLintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err == nil {
		t.Fatal("expected error when CustomLintToolSpec is configured without CustomGCL")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestLintRejectsMalformedCustomLintToolSpec(t *testing.T) {
	t.Parallel()
	_, err := NewLintToolchain(
		LintConfig(".golangci.yml"),
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
		CustomGCL(".custom-gcl.yml"),
		CustomLintToolSpec("not-a-valid-spec"),
	)
	if err == nil {
		t.Fatal("expected malformed CustomLintToolSpec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestDeadcodeRequiresMissingToolSpec(t *testing.T) {
	pkgScope, err := NewPackageScope("./...")
	if err != nil {
		t.Fatalf("NewPackageScope() failed: %v", err)
	}
	err = Deadcode(
		context.Background(),
		noopGoFakeRunner(),
		gatetest.NewFakeToolResolver(),
		gatetest.NewMemoryFileOps(),
		".",
		pkgScope,
		DeadcodeToolSpec(""),
	)
	if err == nil {
		t.Fatal("expected error when DeadcodeToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestValidateLintConfigRejectsMalformedToolSpec(t *testing.T) {
	err := validateLintInputs(LintConfig(".golangci.yml"), LintToolSpec("not-a-valid-spec"))
	if err == nil {
		t.Fatal("expected malformed lint tool spec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestValidateDeadcodeConfigRejectsMalformedToolSpec(t *testing.T) {
	err := validateDeadcodeToolSpec(DeadcodeToolSpec("still-not-valid"))
	if err == nil {
		t.Fatal("expected malformed deadcode tool spec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMarkdownLintRequiresMissingToolSpec(t *testing.T) {
	err := MarkdownLint(
		context.Background(),
		noopGoFakeRunner(),
		gatetest.NewFakeToolResolver(),
		gatetest.NewMemoryFileOps(),
		".",
		MarkdownLintToolSpec(""),
	)
	if err == nil {
		t.Fatal("expected error when MarkdownLintToolSpec is not provided")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestValidateMarkdownLintConfigRejectsMalformedToolSpec(t *testing.T) {
	err := validateMarkdownLintToolSpec(MarkdownLintToolSpec("still-not-valid"))
	if err == nil {
		t.Fatal("expected malformed markdownlint tool spec to be rejected")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}
