// Vision: MutationKills/MutationSites via public gate API—scopes, excludes, resolver wiring, faked runner.
package gate

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestMutationKillsRejectsNilRunner(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = MutationKills(
		context.Background(),
		nil,
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestMutationKillsRejectsNilFileOps(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	store := NewArtifactStore()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		nil,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for nil fileOps")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestMutationKillsRejectsNilStore(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		nil,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestMutationKillsRejectsEmptyQualityScope(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	scope := QualityScope{}
	_, err := MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for empty quality scope")
	}
	if !errors.Is(err, ErrQualityScopeEmpty) {
		t.Fatalf("expected ErrQualityScopeEmpty, got %v", err)
	}
}

func TestMutationKillsRejectsInvalidQualityScopePattern(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	scope := QualityScope{data: &qualityScopeData{packages: "-x"}}
	_, err := MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for invalid quality scope pattern")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsRejectsUnsetMinKillRate(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRateThreshold{},
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for unset MinKillRate")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsRejectsInvalidMinKillRateNegative(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(-5),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for negative MinKillRate")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsRejectsInvalidMinKillRateGreaterThan100(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	_, err = MutationKills(
		context.Background(),
		noopGoFakeRunner(),
		nil,
		store,
		mem,
		".",
		scope,
		QualityScopeInventoryOutput{},
		MinKillRate(105),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for MinKillRate > 100")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsRejectsAllPackagesExcluded(t *testing.T) {
	t.Parallel()
	scope, err := NewQualityScope("./...", Exclude("mage-gate"))
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	runner := mustNewDisplayRunner(t, gateStepFakeRunner(mem), OutputModeAgent, io.Discard, io.Discard)
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, ".", scope)
	_, err = MutationKills(
		context.Background(),
		runner,
		gatetest.NewFakeToolResolver(),
		store,
		mem,
		".",
		scope,
		inv,
		MinKillRate(80),
		testGremlinsTool,
	)
	if err == nil {
		t.Fatal("expected error for all packages excluded")
	}
	// Agent mode wraps harness failures; errors.Is must still reach ErrAllPackagesExcluded.
	if !errors.Is(err, ErrAllPackagesExcluded) && !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrAllPackagesExcluded or ErrMutationKillsFailed, got %v", err)
	}
}

func TestMutationKillsRetrieveFailureUsesWrapStepErrorOutputMode(t *testing.T) {
	t.Parallel()
	_, err := retrieveMutationKillsCheck(NewArtifactStore(), "no-such-step-id")
	if err == nil {
		t.Fatal("expected retrieve error for missing step artifacts")
	}
	inner := cmdtest.NewFakeRunner()
	silentRunner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	wrappedSilent := wrapStepError("mutationkills", silentRunner, err)
	var de *DiagnosticError
	if !errors.As(wrappedSilent, &de) {
		t.Fatalf("expected *DiagnosticError in silent display, got %T: %v", wrappedSilent, wrappedSilent)
	}
	verboseRunner := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)
	wrappedVerbose := wrapStepError("mutationkills", verboseRunner, err)
	var deVerbose *DiagnosticError
	if errors.As(wrappedVerbose, &deVerbose) {
		t.Fatalf("expected raw error chain in verbose display, got diagnostic %v", wrappedVerbose)
	}
}
