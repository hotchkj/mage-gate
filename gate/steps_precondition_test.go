// Vision: Illegal inputs fail before subprocesses (nil deps, missing lint config where required).
package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestMutationSitesRejectsNilStore(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, "./...")
	scanOut := MutationScanOutput{
		stepID:       "mutationscan-1",
		qualityScope: scope,
		outputMode:   OutputModeVerbose,
	}
	err := MutationSites(
		scanOut,
		MaxSites(50),
	)
	if err == nil {
		t.Fatal("expected error for nil Store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestTestRejectsNilStore(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, "./...")
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	_, err = Test(context.Background(), noopGoFakeRunner(), nil, gatetest.NewMemoryFileOps(), ".", pkgScope)
	if err == nil {
		t.Fatal("expected error for nil Store")
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestPublicStepsRejectNilRunner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := "."
	scope := mustNewQualityScope(t, "./...")
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	token := CoveredTestOutput{stepID: "upstream", packages: pkgScope, qualityScope: scope}
	covOut := CoverageOutput{stepID: "cov", qualityScope: scope}
	testOut := TestOutput{stepID: "test-step", scope: pkgScope}
	cases := []struct {
		name string
		fn   func() error
	}{
		{"Lint", func() error {
			return Lint(ctx, nil, nil, mem, root, pkgScope, testDefaultLintToolchain(t))
		}},
		{"Compile", func() error { return Compile(ctx, nil, mem, root, pkgScope) }},
		{"Vet", func() error { return Vet(ctx, nil, mem, root, pkgScope) }},
		{"Deadcode", func() error {
			return Deadcode(
				ctx,
				nil,
				nil,
				mem,
				root,
				pkgScope,
				DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
				DeadcodeArgs("-tags=test"),
			)
		}},
		{"MarkdownLint", func() error {
			return MarkdownLint(
				ctx,
				nil,
				nil,
				mem,
				root,
				MarkdownLintToolSpec("github.com/shinagawa-web/gomarklint/v3@v3.2.3"),
				MarkdownLintArgs("--config", ".gomarklint.json"),
			)
		}},
		{"Test", func() error { _, err := Test(ctx, nil, store, mem, root, pkgScope); return err }},
		{"Coverage", func() error { _, err := Coverage(ctx, nil, store, mem, root, token, MinPercent(90)); return err }},
		{"Crap", func() error {
			return Crap(ctx, nil, nil, store, mem, root, covOut, QualityScopeInventoryOutput{}, MaxScore(8), testGocycloTool)
		}},
		{"Duration", func() error { return Duration(ctx, nil, store, mem, root, testOut, MaxSeconds(5)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.fn(), ErrNilDependency) {
				t.Fatalf("expected ErrNilDependency for %s with nil runner", tc.name)
			}
		})
	}
}

func TestPublicStepsRejectNilFileOps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := "."
	scope := mustNewQualityScope(t, "./...")
	pkgScope, err := NewPackageScope(scope.Packages())
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	runner := noopGoFakeRunner()
	store := NewArtifactStore()
	token := CoveredTestOutput{stepID: "upstream", packages: pkgScope, qualityScope: scope}
	covOut := CoverageOutput{stepID: "cov", qualityScope: scope}
	testOut := TestOutput{stepID: "test-step", scope: pkgScope}
	resolver := gatetest.NewFakeToolResolver()

	cases := []struct {
		name string
		fn   func() error
	}{
		{"Lint", func() error {
			return Lint(ctx, runner, resolver, nil, root, pkgScope, testDefaultLintToolchain(t))
		}},
		{"Compile", func() error { return Compile(ctx, runner, nil, root, pkgScope) }},
		{"Vet", func() error { return Vet(ctx, runner, nil, root, pkgScope) }},
		{"Deadcode", func() error {
			return Deadcode(
				ctx,
				runner,
				resolver,
				nil,
				root,
				pkgScope,
				DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
				DeadcodeArgs("-tags=test"),
			)
		}},
		{"MarkdownLint", func() error {
			return MarkdownLint(
				ctx,
				runner,
				resolver,
				nil,
				root,
				MarkdownLintToolSpec("github.com/shinagawa-web/gomarklint/v3@v3.2.3"),
				MarkdownLintArgs("--config", ".gomarklint.json"),
			)
		}},
		{"Test", func() error { _, err := Test(ctx, runner, store, nil, root, pkgScope); return err }},
		{"Coverage", func() error { _, err := Coverage(ctx, runner, store, nil, root, token, MinPercent(90)); return err }},
		{"Crap", func() error {
			return Crap(
				ctx, runner, resolver, store, nil, root, covOut, QualityScopeInventoryOutput{}, MaxScore(8), testGocycloTool,
			)
		}},
		{"Duration", func() error { return Duration(ctx, runner, store, nil, root, testOut, MaxSeconds(5)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.fn(), ErrNilDependency) {
				t.Fatalf("expected ErrNilDependency for %s with nil fileOps", tc.name)
			}
		})
	}
}

func TestPublicStepsRejectNilResolver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := "."
	pkgScope := mustNewPackageScope(t, "./...")
	runner := noopGoFakeRunner()
	mem := gatetest.NewMemoryFileOps()

	cases := []struct {
		name string
		fn   func() error
	}{
		{"Lint", func() error {
			return Lint(ctx, runner, nil, mem, root, pkgScope, testDefaultLintToolchain(t))
		}},
		{"Deadcode", func() error {
			return Deadcode(
				ctx,
				runner,
				nil,
				mem,
				root,
				pkgScope,
				DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"),
				DeadcodeArgs("-tags=test"),
			)
		}},
		{"MarkdownLint", func() error {
			return MarkdownLint(
				ctx,
				runner,
				nil,
				mem,
				root,
				MarkdownLintToolSpec("github.com/shinagawa-web/gomarklint/v3@v3.2.3"),
				MarkdownLintArgs("--config", ".gomarklint.json"),
			)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.fn(), ErrNilDependency) {
				t.Fatalf("expected ErrNilDependency for %s with nil resolver", tc.name)
			}
		})
	}
}

func TestPublicStepsRejectEmptyOrWhitespaceRoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	runner := noopGoFakeRunner()
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, "./...")
	pkgScope := mustNewPackageScope(t, "./...")
	covOut := CoverageOutput{stepID: "cov", qualityScope: scope}
	testOut := TestOutput{stepID: "test-step", scope: pkgScope}
	resolver := gatetest.NewFakeToolResolver()
	lt := testDefaultLintToolchain(t)

	cases := emptyRootCases(ctx, runner, resolver, mem, store, scope, pkgScope, covOut, testOut, &lt)

	for _, root := range []string{"", "   "} {
		suffix := "/empty"
		if root != "" {
			suffix = "/whitespace"
		}
		for _, tc := range cases {
			t.Run(tc.name+suffix, func(t *testing.T) {
				t.Parallel()
				err := tc.fn(root)
				if err == nil {
					t.Fatalf("expected error for %q root", root)
				}
				if !errors.Is(err, ErrMissingValue) {
					t.Fatalf("expected ErrMissingValue, got %v", err)
				}
			})
		}
	}
}

type emptyRootCase struct {
	name string
	fn   func(root string) error
}

func emptyRootCases(
	ctx context.Context,
	runner CommandRunner,
	resolver ToolResolver,
	mem FileOps,
	store *ArtifactStore,
	scope QualityScope,
	pkgScope PackageScope,
	covOut CoverageOutput,
	testOut TestOutput,
	lt *LintToolchain,
) []emptyRootCase {
	return []emptyRootCase{
		{"Lint", func(root string) error {
			return Lint(ctx, runner, resolver, mem, root, pkgScope, *lt)
		}},
		{"Format", func(root string) error {
			return Format(ctx, runner, resolver, mem, root, pkgScope, *lt)
		}},
		{"Compile", func(root string) error {
			return Compile(ctx, runner, mem, root, pkgScope)
		}},
		{"Vet", func(root string) error {
			return Vet(ctx, runner, mem, root, pkgScope)
		}},
		{"Deadcode", func(root string) error {
			return Deadcode(ctx, runner, resolver, mem, root, pkgScope,
				DeadcodeToolSpec("golang.org/x/tools/cmd/deadcode@v0.31.0"))
		}},
		{"MarkdownLint", func(root string) error {
			return MarkdownLint(ctx, runner, resolver, mem, root,
				MarkdownLintToolSpec("github.com/shinagawa-web/gomarklint/v3@v3.2.3"))
		}},
		{"Test", func(root string) error {
			_, err := Test(ctx, runner, store, mem, root, pkgScope)
			return err
		}},
		{"CoveredTest", func(root string) error {
			_, err := CoveredTest(ctx, runner, store, mem, root,
				pkgScope, scope, QualityScopeInventoryOutput{})
			return err
		}},
		{"QualityScopeInventory", func(root string) error {
			_, err := QualityScopeInventory(ctx, runner, store, mem, root, scope)
			return err
		}},
		{"Coverage", func(root string) error {
			token := CoveredTestOutput{
				stepID: "upstream", packages: pkgScope, qualityScope: scope,
			}
			_, err := Coverage(ctx, runner, store, mem, root, token, MinPercent(90))
			return err
		}},
		{"Crap", func(root string) error {
			return Crap(ctx, runner, resolver, store, mem, root,
				covOut, QualityScopeInventoryOutput{}, MaxScore(8), testGocycloTool)
		}},
		{"Duration", func(root string) error {
			return Duration(ctx, runner, store, mem, root, testOut, MaxSeconds(5))
		}},
		{"MutationKills", func(root string) error {
			_, err := MutationKills(ctx, runner, resolver, store, mem, root,
				scope, QualityScopeInventoryOutput{}, MinKillRate(0), testGremlinsTool)
			return err
		}},
	}
}
