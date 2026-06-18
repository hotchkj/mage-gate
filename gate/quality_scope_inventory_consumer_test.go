// Vision: QualityScopeInventory consumers validate token handles while applying their own runtime filters.
package gate

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestInventoryConsumersRejectInvalidTokensWithoutFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	root := fakeTestModuleRoot
	scope := mustNewQualityScope(t, "./gate/...")
	pkgScope := mustNewPackageScope(t, "./gate/...")
	base := mustQualityScopeInventoryForTests(t, runner, store, mem, root, scope)
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())

	cases := inventoryInvalidCases(store, root, scope, &base)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CoveredTest(ctx, runner, tc.store, mem, tc.root, pkgScope, tc.scope, tc.inventory)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CoveredTest error = %v, want %v", err, tc.want)
			}
			assertQualityScopeInventoryGoListOnce(t, inner.Calls())
		})
	}
}

func TestDirectInventoryConsumersDoNotRediscoverInventoryOnMismatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	root := fakeTestModuleRoot
	scope := mustNewQualityScope(t, "./gate/...")
	pkgScope := mustNewPackageScope(t, "./gate/...")
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, scope)
	setup := inventoryConsumerSetup{
		ctx:      ctx,
		runner:   runner,
		store:    store,
		fileOps:  mem,
		root:     root,
		pkgScope: pkgScope,
		scope:    scope,
		inv:      inv,
	}
	covOut := mustCoverageForInventoryConsumerTests(t, &setup)
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())

	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	wrongRoot := root + "/other"
	cases := []struct {
		name string
		run  func() error
	}{
		{
			name: "crap",
			run: func() error {
				return Crap(
					ctx, runner, resolver, store, mem, wrongRoot,
					covOut, inv, MaxScore(8), testGocycloTool,
				)
			},
		},
		{
			name: "mutation runner scan",
			run: func() error {
				_, err := mr.Scan(ctx, wrongRoot, scope, inv, testGremlinsTool)
				return err
			},
		},
		{
			name: "mutation runner kill",
			run: func() error {
				_, err := mr.Kill(ctx, wrongRoot, scope, inv, testGremlinsTool)
				return err
			},
		},
		{
			name: "mutation kills",
			run: func() error {
				_, err := MutationKills(
					ctx, runner, resolver, store, mem, wrongRoot,
					scope, inv, MinKillRate(0), testGremlinsTool,
				)
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if !errors.Is(err, ErrQualityScopeInventoryMismatch) {
				t.Fatalf("%s error = %v, want ErrQualityScopeInventoryMismatch", tc.name, err)
			}
			assertQualityScopeInventoryGoListOnce(t, inner.Calls())
		})
	}
}

func TestCoveredTestRejectsInventoryWithDifferentScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	root := fakeTestModuleRoot
	inventoryScope, invScopeErr := NewQualityScope("./gate/...", Tags("inventory"))
	if invScopeErr != nil {
		t.Fatalf("NewQualityScope inventory: %v", invScopeErr)
	}
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, inventoryScope)
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())

	pkgScope := mustNewPackageScope(t, internalPackagePattern)
	consumerScope, consumerScopeErr := NewQualityScope(internalPackagePattern, Tags("consumer"))
	if consumerScopeErr != nil {
		t.Fatalf("NewQualityScope consumer: %v", consumerScopeErr)
	}
	_, err := CoveredTest(ctx, runner, store, mem, root, pkgScope, consumerScope, inv)
	if !errors.Is(err, ErrQualityScopeInventoryMismatch) {
		t.Fatalf("CoveredTest error = %v, want ErrQualityScopeInventoryMismatch", err)
	}
}

func TestCoveredTestRejectsInventoryWithDifferentExcludes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	root := fakeTestModuleRoot
	inventoryScope := mustNewQualityScope(t, "./...")
	pkgScope := mustNewPackageScope(t, "./gate/...")
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, inventoryScope)
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())

	unfilteredScope := mustNewQualityScope(t, "./...")
	_, unfilteredErr := CoveredTest(ctx, runner, store, mem, root, pkgScope, unfilteredScope, inv)
	if unfilteredErr != nil {
		t.Fatalf("CoveredTest unfiltered: %v", unfilteredErr)
	}

	filteredScope, err := NewQualityScope("./...", Exclude("internal"))
	if err != nil {
		t.Fatalf("NewQualityScope filtered: %v", err)
	}
	_, filteredErr := CoveredTest(ctx, runner, store, mem, root, pkgScope, filteredScope, inv)
	if !errors.Is(filteredErr, ErrQualityScopeInventoryMismatch) {
		t.Fatalf("CoveredTest filtered error = %v, want ErrQualityScopeInventoryMismatch", filteredErr)
	}
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())
}

func TestMutationScanRejectsInventoryWithDifferentTestFilePatterns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	root := fakeTestModuleRoot
	inner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(fakeModulePath, root, map[string]gatetest.PackageListInfo{
			fakeGateGoTestPackage: {
				Dir:     filepath.Join(root, "gate"),
				Test:    []string{"foo_test.go"},
				GoFiles: []string{"foo.go"},
			},
		})),
		cmdtest.On("go run", gatetest.Gremlins(mem, root, fakeGremlinsReport)),
	)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	inventoryScope := mustNewQualityScope(t, "./gate/...")
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, inventoryScope)
	resolver := gatetest.NewFakeToolResolver()
	mr, err := NewMutationRunner(runner, resolver, store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())

	unfilteredScope := mustNewQualityScope(t, "./gate/...")
	_, unfilteredErr := mr.Scan(ctx, root, unfilteredScope, inv, testGremlinsTool)
	if unfilteredErr != nil {
		t.Fatalf("MutationRunner.Scan unfiltered: %v", unfilteredErr)
	}

	filteredScope, err := NewQualityScope("./gate/...", TestFilePatterns("*_test.go"))
	if err != nil {
		t.Fatalf("NewQualityScope filtered: %v", err)
	}
	_, filteredErr := mr.Scan(ctx, root, filteredScope, inv, testGremlinsTool)
	if !errors.Is(filteredErr, ErrQualityScopeInventoryMismatch) {
		t.Fatalf("MutationRunner.Scan filtered error = %v, want ErrQualityScopeInventoryMismatch", filteredErr)
	}
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())
}

type inventoryConsumerSetup struct {
	ctx      context.Context
	runner   CommandRunner
	store    *ArtifactStore
	fileOps  FileOps
	root     string
	pkgScope PackageScope
	scope    QualityScope
	inv      QualityScopeInventoryOutput
}

func mustCoverageForInventoryConsumerTests(
	t *testing.T,
	setup *inventoryConsumerSetup,
) CoverageOutput {
	t.Helper()
	out, err := CoveredTest(
		setup.ctx,
		setup.runner,
		setup.store,
		setup.fileOps,
		setup.root,
		setup.pkgScope,
		setup.scope,
		setup.inv,
	)
	if err != nil {
		t.Fatalf("CoveredTest setup: %v", err)
	}
	covOut, err := Coverage(
		setup.ctx,
		setup.runner,
		setup.store,
		setup.fileOps,
		setup.root,
		mustCoveredTestOutput(t, &out),
		MinPercent(0),
	)
	if err != nil {
		t.Fatalf("Coverage setup: %v", err)
	}
	return covOut
}

type inventoryInvalidCase struct {
	name      string
	inventory QualityScopeInventoryOutput
	store     *ArtifactStore
	root      string
	scope     QualityScope
	want      error
}

func inventoryInvalidCases(
	store *ArtifactStore,
	root string,
	scope QualityScope,
	base *QualityScopeInventoryOutput,
) []inventoryInvalidCase {
	missingArtifact := *base
	missingArtifact.stepID = "missing-inventory-artifact"
	emptyRows := *base
	emptyRows.rows = nil
	wrongSchema, wrongFormat, wrongFingerprint := *base, *base, *base
	wrongSchema.schema++
	wrongFormat.format = "other-format"
	wrongFingerprint.scopeFingerprint = "deadbeef"
	return []inventoryInvalidCase{
		{
			name:      "zero value",
			inventory: QualityScopeInventoryOutput{},
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryInvalid,
		},
		{
			name:      "wrong store",
			inventory: *base,
			store:     NewArtifactStore(),
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryMismatch,
		},
		{
			name:      "wrong root",
			inventory: *base,
			store:     store,
			root:      root + "/other",
			scope:     scope,
			want:      ErrQualityScopeInventoryMismatch,
		},
		{
			name:      "missing artifact",
			inventory: missingArtifact,
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrMissingValue,
		},
		{
			name:      "empty rows",
			inventory: emptyRows,
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryInvalid,
		},
		{
			name:      "wrong schema",
			inventory: wrongSchema,
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryInvalid,
		},
		{
			name:      "wrong format",
			inventory: wrongFormat,
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryInvalid,
		},
		{
			name:      "wrong scope fingerprint",
			inventory: wrongFingerprint,
			store:     store,
			root:      root,
			scope:     scope,
			want:      ErrQualityScopeInventoryMismatch,
		},
	}
}

func TestInventoryConsumersRejectTagArgs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	root := fakeTestModuleRoot
	scope := mustNewQualityScope(t, "./gate/...")
	pkgScope := mustNewPackageScope(t, "./gate/...")
	inv := mustQualityScopeInventoryForTests(t, runner, store, mem, root, scope)
	_, testErr := CoveredTest(
		ctx, runner, store, mem, root, pkgScope, scope, inv, TestArgs("-tags=unit"),
	)
	if !errors.Is(testErr, ErrInvalidOption) {
		t.Fatalf("CoveredTest tag arg error = %v, want ErrInvalidOption", testErr)
	}
	mr, err := NewMutationRunner(runner, gatetest.NewFakeToolResolver(), store, mem)
	if err != nil {
		t.Fatalf("NewMutationRunner: %v", err)
	}
	_, scanErr := mr.Scan(
		ctx, root, scope, inv, testGremlinsTool, MutationArgs("--tags=unit"),
	)
	if !errors.Is(scanErr, ErrInvalidOption) {
		t.Fatalf("MutationRunner.Scan tag arg error = %v, want ErrInvalidOption", scanErr)
	}
	_, killErr := mr.Kill(
		ctx, root, scope, inv, testGremlinsTool, MutationArgs("-tags=unit"),
	)
	if !errors.Is(killErr, ErrInvalidOption) {
		t.Fatalf("MutationRunner.Kill tag arg error = %v, want ErrInvalidOption", killErr)
	}
	_, mutationKillsErr := MutationKills(
		ctx, runner, gatetest.NewFakeToolResolver(), store, mem, root,
		scope, inv, MinKillRate(0), testGremlinsTool, MutationArgs("--tags=unit"),
	)
	if !errors.Is(mutationKillsErr, ErrInvalidOption) {
		t.Fatalf("MutationKills tag arg error = %v, want ErrInvalidOption", mutationKillsErr)
	}
}
