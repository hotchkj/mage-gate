// Vision: Mutation steps use [QualityScope] only for package pattern and excludes (no per-step mutation drift).
package gate

import "testing"

func TestMutationQualityScopeExcludesOnly(t *testing.T) {
	t.Parallel()

	scope, err := NewQualityScope("./...",
		Exclude("third-party", "vendor"),
	)
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}

	if len(scope.ExcludeSegments()) != 2 {
		t.Fatalf("expected 2 exclude segments in scope, got %d", len(scope.ExcludeSegments()))
	}
	wantExcludes := []string{"third-party", "vendor"}
	gotExcludes := qualityScopeExcludeSegments(scope)
	if len(gotExcludes) != len(wantExcludes) {
		t.Fatalf("unexpected exclude segments: %v", gotExcludes)
	}
	for i := range wantExcludes {
		if gotExcludes[i] != wantExcludes[i] {
			t.Fatalf("unexpected exclude segments: %v", gotExcludes)
		}
	}
}

func TestMutationQualityScopePackagesUsedAsPattern(t *testing.T) {
	t.Parallel()

	scope, err := NewQualityScope("./internal/...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}

	if qualityScopePackages(scope) != "./internal/..." {
		t.Fatalf("expected scope.packages %q, got %q", "./internal/...", qualityScopePackages(scope))
	}
}

func TestQualityScopeTagsAccumulate(t *testing.T) {
	t.Parallel()

	scope, err := NewQualityScope("./...", Tags("mage"), Tags("integration"))
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}

	want := []string{"mage", "integration"}
	got := scope.Tags()
	if len(got) != len(want) {
		t.Fatalf("Tags() got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tags()[%d] got %q want %q", i, got[i], want[i])
		}
	}
	got[0] = "changed"
	if scope.Tags()[0] != "mage" {
		t.Fatalf("Tags() must return a defensive copy, got %v", scope.Tags())
	}
}

func TestMutationArgsAccumulate(t *testing.T) {
	t.Parallel()
	cfg := defaultMutationConfig()
	MutationArgs("--timeout=1m")(&cfg)
	MutationArgs("--workers=2")(&cfg)
	want := []string{"--timeout=1m", "--workers=2"}
	if len(cfg.mutationArgs) != len(want) {
		t.Fatalf("got %v", cfg.mutationArgs)
	}
	for i := range want {
		if cfg.mutationArgs[i] != want[i] {
			t.Fatalf("idx %d: got %q want %q", i, cfg.mutationArgs[i], want[i])
		}
	}
}
