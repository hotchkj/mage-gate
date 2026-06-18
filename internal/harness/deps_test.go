// Vision: StepRunner dependency validation: illegal combinations caught before any command runs (no IO).
package harness_test

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/gatecheck"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

const (
	testPackages            = "./..."
	testRunStepID           = "test-run"
	testStepTestID          = "test-step"
	testStepCovID           = "cov-step"
	testStepCrapID          = "crap-step"
	testLintConfig          = ".golangci.yml"
	testCoverExcludeVendor  = "vendor"
	testCoverageMixedVendor = "mode: set\n" +
		"github.com/foo/vendor/x.go:1.2,3.4 1 1\n" +
		"github.com/hotchkj/mage-gate/internal/harness/config.go:1.2,3.4 1 1\n"
	testStoreArtifactCoverage = "coverage.out"
	testFmtStoreWrite         = "store write: %v"
	testFmtNewHarness         = "NewStepRunner: %v"
	testImportListExample     = "example.com/mod/pkg\n"
	testGoTestJSONLEventLine  = `{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"pkg/a","Elapsed":0.1}`
)

var errSimulatedFailure = errors.New("simulated failure")

var errArtifactNotFound = errors.New("artifact not found")

type memStore struct {
	data       map[string]map[string][]byte
	provenance map[string]map[string]h.Provenance
}

func newMemStore() *memStore {
	return &memStore{
		data:       make(map[string]map[string][]byte),
		provenance: make(map[string]map[string]h.Provenance),
	}
}

func (ms *memStore) Write(stepID, name string, data []byte, prov h.Provenance) error {
	if ms.data[stepID] == nil {
		ms.data[stepID] = make(map[string][]byte)
		ms.provenance[stepID] = make(map[string]h.Provenance)
	}
	ms.data[stepID][name] = append([]byte(nil), data...)
	ms.provenance[stepID][name] = prov
	return nil
}

func (ms *memStore) Read(stepID, name string) ([]byte, error) {
	if byName, ok := ms.data[stepID]; ok {
		if data, ok := byName[name]; ok {
			return append([]byte(nil), data...), nil
		}
	}
	return nil, errArtifactNotFound
}

func (ms *memStore) Has(stepID, name string) bool {
	if byName, ok := ms.data[stepID]; ok {
		_, ok := byName[name]
		return ok
	}
	return false
}

// Provenance returns the Provenance recorded for a stored artifact.
func (ms *memStore) Provenance(stepID, name string) (h.Provenance, bool) {
	if byName, ok := ms.provenance[stepID]; ok {
		if prov, ok := byName[name]; ok {
			return prov, true
		}
	}
	return h.Provenance{}, false
}

const (
	testHarnessRoot           = "/test-root"
	testHarnessArtifactSubdir = "test-artifacts"
	// Logical artifact-relative paths matching harness artifactPaths under testHarnessArtifactSubdir (forward slashes).
	testHarnessCoverageLogicalRel         = testHarnessArtifactSubdir + "/" + testStoreArtifactCoverage
	testHarnessCoverageFilteredLogicalRel = testHarnessArtifactSubdir + "/coverage-filtered.out"
	testHarnessGocycloLogicalRel          = testHarnessArtifactSubdir + "/gocyclo.txt"
	testHarnessCoverFuncLogicalRel        = testHarnessArtifactSubdir + "/cover-func.txt"
	testHarnessMutationsLogicalRel        = testHarnessArtifactSubdir + "/mutations.json"
)

func testHarnessHostDir(tb testing.TB, parts ...string) string {
	tb.Helper()
	root, err := filepath.Abs(testHarnessRoot)
	if err != nil {
		tb.Fatal(err)
	}
	joined := append([]string{root}, parts...)
	return filepath.Join(joined...)
}

func testHarnessPkgDir(tb testing.TB) string {
	tb.Helper()
	return testHarnessHostDir(tb, "pkg")
}

func testMutationInventoryRows(tb testing.TB) []gatecheck.MutationPackageRow {
	tb.Helper()
	return []gatecheck.MutationPackageRow{
		{
			ImportPath:    "example.com/mod/pkg",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"foo.go"},
		},
	}
}

func testQualityScopeCommandScope(
	rows []gatecheck.MutationPackageRow,
	rawExclude string,
	testFilePatterns []string,
	tags []string,
) *gatecheck.QualityScopeCommandScope {
	p := gatecheck.NewQualityScopeCommandScope(rows, nil, rawExclude, testFilePatterns, tags)
	return &p
}

func testEmptyQualityScopeCommandScope() *gatecheck.QualityScopeCommandScope {
	return testQualityScopeCommandScope(nil, "", nil, nil)
}

func testMutationCommandScope(tb testing.TB, rawExclude string) *gatecheck.QualityScopeCommandScope {
	tb.Helper()
	return testQualityScopeCommandScope(testMutationInventoryRows(tb), rawExclude, nil, nil)
}

// testHarnessCustomLintExeLogicalCmdKey is the command-name key FakeRunner uses for the artifact-resident
// custom golangci-lint binary argv (canonical logical path under cwd root, OS-specific exe suffix).
func testHarnessCustomLintExeLogicalCmdKey() string {
	s := testHarnessArtifactSubdir + "/custom-gcl"
	if runtime.GOOS == goosWindows {
		return s + ".exe"
	}
	return s
}

// memFileOpsMkdirFail wraps NewMemoryFileOps, forcing MkdirAll to fail.
type memFileOpsMkdirFail struct {
	inner h.FileOps
	mkErr error
}

func newMemFileOpsMkdirFail(mkErr error) *memFileOpsMkdirFail {
	inner := gatetest.NewMemoryFileOps()
	if err := inner.Root(testHarnessRoot); err != nil {
		panic("Root for memFileOpsMkdirFail inner: " + err.Error())
	}
	return &memFileOpsMkdirFail{inner: inner, mkErr: mkErr}
}

func (m *memFileOpsMkdirFail) Root(root string) error {
	return m.inner.Root(root)
}

func (m *memFileOpsMkdirFail) MkdirAll(string, fs.FileMode) error {
	return m.mkErr
}

func (m *memFileOpsMkdirFail) MkdirTemp(dir, pattern string) (string, error) {
	return m.inner.MkdirTemp(dir, pattern)
}

func (m *memFileOpsMkdirFail) RemoveAll(path string) error {
	return m.inner.RemoveAll(path)
}

func (m *memFileOpsMkdirFail) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return m.inner.WriteFile(path, data, perm)
}

func (m *memFileOpsMkdirFail) ReadFile(path string) ([]byte, error) {
	return m.inner.ReadFile(path)
}

func (m *memFileOpsMkdirFail) CreateFile(path string) (io.WriteCloser, error) {
	return m.inner.CreateFile(path)
}

func (m *memFileOpsMkdirFail) Walk(root string, fn filepath.WalkFunc) error {
	return m.inner.Walk(root, fn)
}

// memFileOpsWalkFail wraps NewMemoryFileOps, forcing Walk to fail before FileOps traversal
// (used to assert mutation steps fail closed before invoking go list).
type memFileOpsWalkFail struct {
	inner   h.FileOps
	walkErr error
}

func newMemFileOpsWalkFail(walkErr error) *memFileOpsWalkFail {
	inner := gatetest.NewMemoryFileOps()
	if err := inner.Root(testHarnessRoot); err != nil {
		panic("Root for memFileOpsWalkFail inner: " + err.Error())
	}
	return &memFileOpsWalkFail{inner: inner, walkErr: walkErr}
}

func (m *memFileOpsWalkFail) Root(root string) error {
	return m.inner.Root(root)
}

func (m *memFileOpsWalkFail) MkdirAll(path string, mode fs.FileMode) error {
	return m.inner.MkdirAll(path, mode)
}

func (m *memFileOpsWalkFail) MkdirTemp(dir, pattern string) (string, error) {
	return m.inner.MkdirTemp(dir, pattern)
}

func (m *memFileOpsWalkFail) RemoveAll(path string) error {
	return m.inner.RemoveAll(path)
}

func (m *memFileOpsWalkFail) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return m.inner.WriteFile(path, data, perm)
}

func (m *memFileOpsWalkFail) ReadFile(path string) ([]byte, error) {
	return m.inner.ReadFile(path)
}

func (m *memFileOpsWalkFail) CreateFile(path string) (io.WriteCloser, error) {
	return m.inner.CreateFile(path)
}

func (m *memFileOpsWalkFail) Walk(string, filepath.WalkFunc) error {
	return m.walkErr
}

func validFileOps() h.FileOps {
	m := gatetest.NewMemoryFileOps()
	if err := m.Root(testHarnessRoot); err != nil {
		panic("validFileOps Root: " + err.Error())
	}
	return m
}

type testHarnessDeps struct {
	Runner  cmdrunner.CommandRunner
	FileOps h.FileOps
	Options []h.StepRunnerOption
}

func validDeps(runner cmdrunner.CommandRunner) testHarnessDeps {
	return testHarnessDeps{
		Runner:  runner,
		FileOps: validFileOps(),
		Options: []h.StepRunnerOption{h.WithToolResolver(gatetest.NewFakeToolResolver().SetDefaultToLocal(true))},
	}
}

func newTestHarness(
	root, packages string,
	deps testHarnessDeps,
	store h.ArtifactStore,
	stepID string,
) (*h.StepRunner, error) {
	return h.NewStepRunner(
		root,
		testHarnessArtifactSubdir,
		packages,
		deps.Runner,
		deps.FileOps,
		store,
		stepID,
		deps.Options...,
	)
}

func TestNewStepRunnerRejectsNilRunner(t *testing.T) {
	t.Parallel()
	_, err := h.NewStepRunner(
		testHarnessRoot,
		testHarnessArtifactSubdir,
		testPackages,
		nil,
		validFileOps(),
		h.NewDiscardArtifactStore(),
		"",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDepsRequired) {
		t.Fatalf("expected ErrDepsRequired, got %v", err)
	}
}

func TestNewStepRunnerRejectsNilFileOps(t *testing.T) {
	t.Parallel()
	_, err := h.NewStepRunner(
		testHarnessRoot,
		testHarnessArtifactSubdir,
		testPackages,
		cmdtest.NewFakeRunner(),
		nil,
		h.NewDiscardArtifactStore(),
		"",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDepsRequired) {
		t.Fatalf("expected ErrDepsRequired, got %v", err)
	}
}

func TestNewStepRunnerAllowsNoToolResolver(t *testing.T) {
	t.Parallel()
	_, err := h.NewStepRunner(
		testHarnessRoot,
		testHarnessArtifactSubdir,
		testPackages,
		cmdtest.NewFakeRunner(),
		validFileOps(),
		h.NewDiscardArtifactStore(),
		"",
	)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
