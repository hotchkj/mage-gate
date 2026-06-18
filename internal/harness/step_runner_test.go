// Vision: StepRunner construction, lifecycle cleanup, and error propagation with a faked underlying runner.
package harness_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

func TestNewHarness_EmptyRoot(t *testing.T) {
	t.Parallel()
	_, err := newTestHarness("", testPackages, validDeps(cmdtest.NewFakeRunner()), h.NewDiscardArtifactStore(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrRootRequired) {
		t.Fatalf("expected ErrRootRequired, got %v", err)
	}
}

func TestNewHarness_NilStore(t *testing.T) {
	t.Parallel()
	_, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrArtifactStoreRequired) {
		t.Fatalf("expected ErrArtifactStoreRequired, got %v", err)
	}
}

func TestNewHarness_InvalidDeps(t *testing.T) {
	t.Parallel()
	deps := validDeps(cmdtest.NewFakeRunner())
	deps.Runner = nil
	_, err := newTestHarness(testHarnessRoot, testPackages, deps, h.NewDiscardArtifactStore(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDepsRequired) {
		t.Fatalf("expected ErrDepsRequired, got %v", err)
	}
}

func TestNewHarness_EmptyArtifactSubdirCreatesTempDir(t *testing.T) {
	t.Parallel()
	harn, err := h.NewStepRunner(
		testHarnessRoot,
		"",
		testPackages,
		validDeps(cmdtest.NewFakeRunner()).Runner,
		validDeps(cmdtest.NewFakeRunner()).FileOps,
		h.NewDiscardArtifactStore(),
		"",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	if harn == nil {
		t.Fatal("nil harness")
	}
}

func TestNewHarness_OK(t *testing.T) {
	t.Parallel()
	harness, err := newTestHarness(
		testHarnessRoot,
		testPackages,
		validDeps(cmdtest.NewFakeRunner()),
		h.NewDiscardArtifactStore(),
		"",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	if harness == nil {
		t.Fatal("nil harness")
	}
}

func TestCleanup_TempOwnedSucceeds(t *testing.T) {
	t.Parallel()
	harn, err := h.NewStepRunner(
		testHarnessRoot, "", testPackages,
		cmdtest.NewFakeRunner(), validFileOps(), h.NewDiscardArtifactStore(), "",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	if dir := h.ArtifactSubdirForTest(harn); dir == "" {
		t.Fatal("expected non-empty artifact subdir")
	}
	if err := harn.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_ConsumerDirIsNoop(t *testing.T) {
	t.Parallel()
	harn, err := h.NewStepRunner(
		testHarnessRoot, testHarnessArtifactSubdir,
		testPackages, cmdtest.NewFakeRunner(), validFileOps(), h.NewDiscardArtifactStore(), "",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	if err := harn.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_CalledTwiceNoError(t *testing.T) {
	t.Parallel()
	harn, err := h.NewStepRunner(
		testHarnessRoot, "", testPackages,
		cmdtest.NewFakeRunner(), validFileOps(), h.NewDiscardArtifactStore(), "",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	if err := harn.Cleanup(); err != nil {
		t.Fatalf("first Cleanup: %v", err)
	}
	if err := harn.Cleanup(); err != nil {
		t.Fatalf("second Cleanup: %v", err)
	}
}
