// Vision: Primary go test step: argv for packages/shuffle/cover, artifact capture, and failures under fakes.
package harness_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

// canonicalCoverprofileFromGoTest asserts -coverprofile matches artifactPaths.CommandPath("coverage.out")
// for the harness (FileOps reads the profile via FileOpsPath separately when those projections differ).
func canonicalCoverprofileFromGoTest(tb testing.TB, cmd cmdrunner.Command) string {
	tb.Helper()
	covPath, ok := cmd.Go().FlagValue("coverprofile")
	if !ok {
		tb.Fatal("missing -coverprofile flag in go test argv")
	}
	if filepath.IsAbs(covPath) {
		tb.Fatalf("-coverprofile must not be host-absolute, got %q", covPath)
	}
	if filepath.ToSlash(covPath) != filepath.ToSlash(testHarnessCoverageLogicalRel) {
		tb.Fatalf("-coverprofile=%q, want root-relative canonical %q",
			covPath, testHarnessCoverageLogicalRel)
	}
	return covPath
}

func TestStepTest_Success(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, testGoTestJSONLEventLine+"\n")
			prof := []byte("mode: set\nsome/pkg/file.go:1.2,3.4 1 1\n")
			covPath := canonicalCoverprofileFromGoTest(t, cmd)
			return fops.WriteFile(covPath, prof, 0o600)
		}),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepTestID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepTest(context.Background(), testPackages, false, "", nil); err != nil {
		t.Fatalf("StepTest: %v", err)
	}
	assertTestEventsStored(t, store, testStepTestID)
	if !store.Has(testStepTestID, testStoreArtifactCoverage) {
		t.Fatal("expected coverage.out in store")
	}
}

// assertTestEventsStored verifies that the test-events.jsonl artifact is stored,
// contains valid JSONL, and has correct provenance.
func assertTestEventsStored(t *testing.T, store *memStore, stepID string) {
	t.Helper()
	if !store.Has(stepID, "test-events.jsonl") {
		t.Fatal("expected test-events.jsonl in store")
	}
	content, readErr := store.Read(stepID, "test-events.jsonl")
	if readErr != nil {
		t.Fatalf("store.Read: %v", readErr)
	}
	if len(content) == 0 || content[0] != '{' {
		t.Fatalf("stored content is not valid JSONL, got %q", content)
	}
	prov, ok := store.Provenance(stepID, "test-events.jsonl")
	if !ok {
		t.Fatal("expected provenance for test-events.jsonl")
	}
	if prov.StepID != stepID {
		t.Errorf("Provenance.StepID = %q, want %q", prov.StepID, stepID)
	}
}

func TestStepTest_WithoutCoverpkg(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, testGoTestJSONLEventLine+"\n")
			return nil
		}),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepTestID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepTest(context.Background(), "", false, "", nil); err != nil {
		t.Fatalf("StepTest: %v", err)
	}
	if store.Has(testStepTestID, testStoreArtifactCoverage) {
		t.Error("did not expect coverage.out when Coverpkg is empty")
	}
	if !store.Has(testStepTestID, "test-events.jsonl") {
		t.Error("expected test-events.jsonl in store")
	}
}

func TestStepCoveredTest_Success(t *testing.T) {
	t.Parallel()
	fops := gatetest.NewMemoryFileOps()
	store := newMemStore()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", gatetest.GoList(
			"", "", map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly(testHarnessPkgDir(t))},
		)),
		cmdtest.On("go test", func(_ context.Context, cmd cmdrunner.Command, stdout, _ io.Writer) error {
			_, _ = io.WriteString(stdout, testGoTestJSONLEventLine+"\n")
			prof := []byte("mode: set\nsome/pkg/file.go:1.2,3.4 1 1\n")
			covPath := canonicalCoverprofileFromGoTest(t, cmd)
			return fops.WriteFile(covPath, prof, 0o600)
		}),
	)
	deps := validDeps(runner)
	deps.FileOps = fops
	harness, err := newTestHarness(testHarnessRoot, testPackages, deps, store, testStepTestID)
	if err != nil {
		t.Fatalf(testFmtNewHarness, err)
	}
	if err := harness.StepCoveredTest(
		context.Background(), testQualityScopeCommandScope(testMutationInventoryRows(t), "", nil, nil), false, nil,
	); err != nil {
		t.Fatalf("StepCoveredTest: %v", err)
	}
	if !store.Has(testStepTestID, "test-events.jsonl") {
		t.Error("expected test-events.jsonl in store")
	}
	if !store.Has(testStepTestID, testStoreArtifactCoverage) {
		t.Error("expected coverage.out in store")
	}
}
