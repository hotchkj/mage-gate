// Vision: Functional-option and token validation across steps (illegal combinations, zero values, cross-field rules).
package gate

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

const (
	testGocycloToolSpec  = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"
	testGremlinsToolSpec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
)

var (
	testGocycloTool  = GocycloToolSpec(testGocycloToolSpec)
	testGremlinsTool = GremlinsToolSpec(testGremlinsToolSpec)
)

func TestTestFilePatterns(t *testing.T) {
	scope, err := NewQualityScope("./...", TestFilePatterns("*_test.go", "*_integration.go"))
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	if len(scope.TestFilePatterns()) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(scope.TestFilePatterns()))
	}
	if scope.TestFilePatterns()[0] != "*_test.go" {
		t.Errorf("expected first pattern '*_test.go', got %q", scope.TestFilePatterns()[0])
	}
}

func TestDeadcodeArgsOption(t *testing.T) {
	var cfg deadcodeConfig
	DeadcodeArgs("-tags=integration")(&cfg)
	if len(cfg.args) != 1 || cfg.args[0] != "-tags=integration" {
		t.Errorf("expected args [-tags=integration], got %v", cfg.args)
	}
}

func TestMarkdownLintArgsOption(t *testing.T) {
	cfg := defaultMarkdownlintConfig()
	MarkdownLintArgs("--config", ".gomarklint.json")(&cfg)
	want := []string{"--config", ".gomarklint.json"}
	if len(cfg.args) != len(want) {
		t.Fatalf("args = %v, want %v", cfg.args, want)
	}
	for i := range want {
		if cfg.args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, cfg.args[i], want[i])
		}
	}
}

func TestCustomGCLOption(t *testing.T) {
	var cfg lintConfig
	CustomGCL("/path/to/.custom-gcl.yml")(&cfg)
	if cfg.customGCLPath != "/path/to/.custom-gcl.yml" {
		t.Errorf("expected customGCLPath set, got %q", cfg.customGCLPath)
	}
}

func TestLintRequiresConfig(t *testing.T) {
	_, err := NewLintToolchain(
		LintConfig(""),
		LintToolSpec("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"),
	)
	if err == nil {
		t.Fatal("expected error when LintConfig is not provided")
	}
	if !errors.Is(err, ErrLintConfigRequired) {
		t.Fatalf("expected ErrLintConfigRequired, got %v", err)
	}
}

func TestDeadcodeRejectsEmptyScopePackages(t *testing.T) {
	var pkgScope PackageScope
	err := Deadcode(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		gatetest.NewMemoryFileOps(),
		".",
		pkgScope,
		DeadcodeToolSpec(""),
	)
	if err == nil {
		t.Fatal("expected error when scope packages empty")
	}
	if !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("expected ErrPackageScopeEmpty, got %v", err)
	}
}

func TestCompilePublicRejectsEmptyScope(t *testing.T) {
	err := Compile(context.Background(), noopGoFakeRunner(), gatetest.NewMemoryFileOps(), ".", PackageScope{})
	if !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("expected ErrPackageScopeEmpty, got %v", err)
	}
}

func TestVetPublicRejectsEmptyScope(t *testing.T) {
	err := Vet(context.Background(), noopGoFakeRunner(), gatetest.NewMemoryFileOps(), ".", PackageScope{})
	if !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("expected ErrPackageScopeEmpty, got %v", err)
	}
}

func TestDeadcodePublicRejectsEmptyScope(t *testing.T) {
	err := Deadcode(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		gatetest.NewMemoryFileOps(),
		".",
		PackageScope{},
		DeadcodeToolSpec(""),
	)
	if !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("expected ErrPackageScopeEmpty, got %v", err)
	}
}

func TestCoverageValidation(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	covToken := CoveredTestOutput{stepID: "t", packages: pkgScope, qualityScope: scope}
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", covToken, MinPercent(150))
	if err == nil {
		t.Fatal("expected error for MinPercent above 100")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MinPercent above 100, got %v", err)
	}
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", covToken, MinPercent(-1))
	if err == nil {
		t.Fatal("expected error for negative MinPercent")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for negative MinPercent, got %v", err)
	}
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", covToken, MinPercent(math.NaN()))
	if err == nil {
		t.Fatal("expected error for NaN MinPercent")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for NaN MinPercent, got %v", err)
	}
}

func TestCrapValidation(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	covOut := CoverageOutput{stepID: "c", qualityScope: scope}
	err = Crap(
		context.Background(), noopGoFakeRunner(), nil, store, mem, ".", covOut,
		QualityScopeInventoryOutput{}, MaxScore(-1), testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for invalid MaxScore")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxScore(-1), got %v", err)
	}
	err = Crap(
		context.Background(), noopGoFakeRunner(), nil, store, mem, ".", covOut,
		QualityScopeInventoryOutput{}, MaxScore(0), testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for MaxScore zero")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxScore(0), got %v", err)
	}
	err = Crap(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		covOut,
		QualityScopeInventoryOutput{},
		MaxScore(math.NaN()),
		testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for NaN MaxScore")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for NaN MaxScore, got %v", err)
	}
}

func TestDurationValidation(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	testOut := TestOutput{stepID: "t", scope: pkgScope}
	err = Duration(context.Background(), noopGoFakeRunner(), store, mem, ".", testOut, MaxSeconds(-1))
	if err == nil {
		t.Fatal("expected error for invalid MaxSeconds")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxSeconds(-1), got %v", err)
	}
	err = Duration(context.Background(), noopGoFakeRunner(), store, mem, ".", testOut, MaxSeconds(0))
	if err == nil {
		t.Fatal("expected error for MaxSeconds zero")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxSeconds(0), got %v", err)
	}
	err = Duration(context.Background(), noopGoFakeRunner(), store, mem, ".", testOut, MaxSeconds(math.NaN()))
	if err == nil {
		t.Fatal("expected error for NaN MaxSeconds")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for NaN MaxSeconds, got %v", err)
	}
}

func TestMutationSitesValidation(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	store := NewArtifactStore()
	scanOut := MutationScanOutput{
		store:        store,
		stepID:       "mutationscan-1",
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	err = MutationSites(
		scanOut,
		MaxSites(-1),
	)
	if err == nil {
		t.Fatal("expected error for invalid MaxSites")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxSites(-1), got %v", err)
	}
	err = MutationSites(
		scanOut,
		MaxSites(0),
	)
	if err == nil {
		t.Fatal("expected error for MaxSites zero")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption for MaxSites(0), got %v", err)
	}
}

func TestCoverageRequiresMinPercent(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	covToken := CoveredTestOutput{stepID: "t", packages: pkgScope, qualityScope: scope}
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", covToken, CoverageThreshold{})
	if err == nil {
		t.Fatal("expected error when MinPercent is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestCrapRequiresMaxScore(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	covOut := CoverageOutput{stepID: "c", qualityScope: scope}
	err = Crap(
		context.Background(), noopGoFakeRunner(), nil, store, mem, ".", covOut,
		QualityScopeInventoryOutput{}, CrapThreshold{}, testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error when MaxScore is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestDurationRequiresMaxSeconds(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	testOut := TestOutput{stepID: "t", scope: pkgScope}
	err = Duration(context.Background(), noopGoFakeRunner(), store, mem, ".", testOut, DurationThreshold{})
	if err == nil {
		t.Fatal("expected error when MaxSeconds is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationSitesRequiresMaxSites(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	store := NewArtifactStore()
	scanOut := MutationScanOutput{
		store:        store,
		stepID:       "mutationscan-1",
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	err = MutationSites(
		scanOut,
		MutationSitesThreshold{},
	)
	if err == nil {
		t.Fatal("expected error when MaxSites is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationCoverageRequiresMinMutationCoverage(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope() failed: %v", err)
	}
	store := NewArtifactStore()
	scanOut := MutationScanOutput{
		store:        store,
		stepID:       "m",
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	err = MutationCoverage(
		scanOut,
		MutationCoverageThreshold{},
	)
	if err == nil {
		t.Fatal("expected error when MinMutationCoverage is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestCoverageZeroValueToken(t *testing.T) {
	zeroOutput := CoveredTestOutput{}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err := Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", zeroOutput, MinPercent(90))
	if err == nil {
		t.Fatal("expected error for zero-value CoveredTestOutput token")
	}
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCrapZeroValueToken(t *testing.T) {
	zeroOutput := CoverageOutput{}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	rslv := gatetest.NewFakeToolResolver()
	err := Crap(
		context.Background(), noopGoFakeRunner(), rslv, store, mem, ".", zeroOutput,
		QualityScopeInventoryOutput{}, MaxScore(8), testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for zero-value CoverageOutput token")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestCoverageRejectsZeroStepID(t *testing.T) {
	zeroOutput := CoveredTestOutput{stepID: ""}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err := Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", zeroOutput, MinPercent(90))
	if err == nil {
		t.Fatal("expected error for zero-value stepID")
	}
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCoverageRejectsPartialTokenEmptyPackages(t *testing.T) {
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	out := CoveredTestOutput{stepID: "x", qualityScope: scope}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = Coverage(context.Background(), noopGoFakeRunner(), store, mem, ".", out, MinPercent(90))
	assertErrorIs(t, err, ErrMissingValue)
}

func TestCrapRejectsZeroStepID(t *testing.T) {
	zeroOutput := CoverageOutput{stepID: ""}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	rslv := gatetest.NewFakeToolResolver()
	err := Crap(
		context.Background(), noopGoFakeRunner(), rslv, store, mem, ".", zeroOutput,
		QualityScopeInventoryOutput{}, MaxScore(8), testGocycloTool,
	)
	if err == nil {
		t.Fatal("expected error for zero-value stepID")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestValidationError_MessageAndStep(t *testing.T) {
	t.Parallel()
	err := newValidationError("s", "m", nil)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("not a ValidationError: %v", err)
	}
	if ve.Message() != "m" {
		t.Fatalf("Message: got %q", ve.Message())
	}
	if ve.Step() != "s" {
		t.Fatalf("Step: got %q", ve.Step())
	}
}
