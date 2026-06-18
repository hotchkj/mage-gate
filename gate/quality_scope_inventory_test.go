// Vision: QualityScopeInventory produces a public token and sealed package-inventory artifact.
package gate

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func isQualityScopeInventoryGoList(cmd cmdrunner.Command) bool {
	args := cmd.Args()
	return cmd.Name() == cmdGo &&
		cmd.Arg(0) == cmdList &&
		slices.Contains(args, "-e") &&
		slices.Contains(args, "-f") &&
		slices.Contains(args, gatecheck.QualityScopeListFormat)
}

func countQualityScopeInventoryGoList(calls []cmdrunner.Command) int {
	count := 0
	for _, cmd := range calls {
		if isQualityScopeInventoryGoList(cmd) {
			count++
		}
	}
	return count
}

func TestQualityScopeInventoryStoresArtifactAndTokenRows(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	inner := gateStepFakeRunner(mem)
	runner := mustNewDisplayRunner(t, inner, OutputModeAgent, io.Discard, io.Discard)
	store := NewArtifactStore()
	scope, err := NewQualityScope("./gate/...", Tags("unit"))
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	out, err := QualityScopeInventory(context.Background(), runner, store, mem, fakeTestModuleRoot, scope)
	if err != nil {
		t.Fatalf("QualityScopeInventory: %v", err)
	}
	assertQualityScopeInventoryToken(t, &out, store, scope)
	assertQualityScopeInventoryArtifact(t, store, out.stepID)
	assertQualityScopeInventoryGoListOnce(t, inner.Calls())
}

func assertQualityScopeInventoryToken(
	t *testing.T,
	out *QualityScopeInventoryOutput,
	store *ArtifactStore,
	scope QualityScope,
) {
	t.Helper()
	if out.stepID == "" || out.store != store || out.root != fakeTestModuleRoot {
		t.Fatalf("unexpected inventory token identity: %#v", out)
	}
	if out.schema != qualityScopeInventorySchemaVersion || out.format != gatecheck.QualityScopeListFormat {
		t.Fatalf("unexpected inventory token contract: schema=%d format=%q", out.schema, out.format)
	}
	if len(out.rows) != 1 || out.rows[0].ImportPath != fakeGateGoTestPackage {
		t.Fatalf("unexpected token rows: %#v", out.rows)
	}
	wantFP := qualityScopeFingerprint(scope)
	if out.scopeFingerprint != wantFP {
		t.Fatalf("scopeFingerprint = %q, want %q", out.scopeFingerprint, wantFP)
	}
}

func assertQualityScopeInventoryArtifact(t *testing.T, store *ArtifactStore, wantStepID string) {
	t.Helper()
	stepID, ok := store.FindArtifact(QualityScopeInventoryArtifactName)
	if !ok || stepID != wantStepID {
		t.Fatalf("inventory artifact stepID = %q, found=%v; want %q", stepID, ok, wantStepID)
	}
	data, err := store.Read(stepID, QualityScopeInventoryArtifactName)
	if err != nil {
		t.Fatalf("read inventory artifact: %v", err)
	}
	assertQualityScopeInventoryArtifactRows(t, data)
	assertQualityScopeInventoryProvenance(t, store, stepID, wantStepID)
}

func assertQualityScopeInventoryArtifactRows(t *testing.T, data []byte) {
	t.Helper()
	rows, err := decodeQualityScopeInventoryArtifact(data)
	if err != nil {
		t.Fatalf("decode inventory artifact: %v", err)
	}
	if len(rows) != 1 || rows[0].ImportPath != fakeGateGoTestPackage {
		t.Fatalf("unexpected artifact rows: %#v", rows)
	}
}

func assertQualityScopeInventoryProvenance(
	t *testing.T,
	store *ArtifactStore,
	stepID, wantStepID string,
) {
	t.Helper()
	prov, err := store.ReadProvenance(stepID, QualityScopeInventoryArtifactName)
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	if prov.StepID != wantStepID || prov.Tool != "go list -e" || prov.Packages != "./gate/..." {
		t.Fatalf("unexpected inventory provenance: %#v", prov)
	}
}

func assertQualityScopeInventoryGoListOnce(t *testing.T, calls []cmdrunner.Command) {
	t.Helper()
	if got := countQualityScopeInventoryGoList(calls); got != 1 {
		t.Fatalf("quality inventory go list count = %d, want 1", got)
	}
}

func mustQualityScopeInventoryForTests(
	tb testing.TB,
	runner CommandRunner,
	store *ArtifactStore,
	fileOps FileOps,
	root string,
	scope QualityScope,
) QualityScopeInventoryOutput {
	tb.Helper()
	out, err := QualityScopeInventory(context.Background(), runner, store, fileOps, root, scope)
	if err != nil {
		tb.Fatalf("QualityScopeInventory: %v", err)
	}
	return out
}

func TestQualityScopeInventoryRejectsDependenciesAndScope(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	store := NewArtifactStore()
	scope := mustNewQualityScope(t, "./...")
	cases := []struct {
		name   string
		runner CommandRunner
		store  *ArtifactStore
		fileOp FileOps
		scope  QualityScope
		want   error
	}{
		{name: "nil runner", runner: nil, store: store, fileOp: mem, scope: scope, want: ErrNilDependency},
		{name: "nil store", runner: noopGoFakeRunner(), store: nil, fileOp: mem, scope: scope, want: ErrNilDependency},
		{name: "nil file ops", runner: noopGoFakeRunner(), store: store, fileOp: nil, scope: scope, want: ErrNilDependency},
		{
			name:   "empty scope",
			runner: noopGoFakeRunner(),
			store:  store,
			fileOp: mem,
			scope:  QualityScope{},
			want:   ErrQualityScopeEmpty,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := QualityScopeInventory(
				context.Background(),
				tc.runner,
				tc.store,
				tc.fileOp,
				fakeTestModuleRoot,
				tc.scope,
			)
			if !errors.Is(err, tc.want) {
				t.Fatalf("QualityScopeInventory error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestQualityScopeInventoryWrapsGoListFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fake cmdtest.CommandFunc
	}{
		{name: "command failure", fake: gatetest.Fail(errForcedFailure)},
		{name: "invalid row", fake: func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, err := io.WriteString(stdout, "not\tan\tinventory\trow\n")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := gatetest.NewMemoryFileOps()
			inner := cmdtest.NewFakeRunner(cmdtest.On("go list", tc.fake))
			runner := mustNewDisplayRunner(t, inner, OutputModeVerbose, io.Discard, io.Discard)
			scope := mustNewQualityScope(t, "./...")
			_, err := QualityScopeInventory(
				context.Background(),
				runner,
				NewArtifactStore(),
				mem,
				fakeTestModuleRoot,
				scope,
			)
			if !errors.Is(err, ErrQualityScopeInventoryFailed) {
				t.Fatalf("QualityScopeInventory error = %v, want ErrQualityScopeInventoryFailed", err)
			}
		})
	}
}
