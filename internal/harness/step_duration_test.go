// Vision: Duration step: test2json stream handling, per-test timing, and max-duration enforcement under fakes.
package harness_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	h "github.com/hotchkj/mage-gate/internal/harness"
)

func durationTestEvents(pkg string, testElapsed, pkgElapsed float64) string {
	return `{"Action":"run","Package":"` + pkg + `","Test":"TestPass"}` + "\n" +
		fmt.Sprintf(`{"Action":"pass","Package":%q,"Test":"TestPass","Elapsed":%v}`, pkg, testElapsed) + "\n" +
		fmt.Sprintf(`{"Action":"pass","Package":%q,"Elapsed":%v}`, pkg, pkgElapsed)
}

func TestStepDuration_ZeroMaxSecondsIsError(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDuration(context.Background(), 0, testRunStepID)
	if err == nil {
		t.Fatal("expected error for UnitMaxSeconds=0")
	}
	if !errors.Is(err, h.ErrDurationFailed) {
		t.Fatalf("expected ErrDurationFailed, got %v", err)
	}
}

func TestStepDuration_MissingUpstreamStepID(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDuration(context.Background(), 5.0, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDurationFailed) {
		t.Fatalf("expected ErrDurationFailed, got %v", err)
	}
}

func TestStepDuration_MissingStoreData(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDuration(context.Background(), 5.0, testRunStepID)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDurationFailed) {
		t.Fatalf("expected ErrDurationFailed, got %v", err)
	}
}

func TestStepDuration_Passing(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	events := durationTestEvents("pkg/a", 0.5, 10.0)
	if err := store.Write(testRunStepID, "test-events.jsonl", []byte(events), h.Provenance{}); err != nil {
		t.Fatalf("store write: %v", err)
	}
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	if err := harness.StepDuration(context.Background(), 5.0, testRunStepID); err != nil {
		t.Fatalf("StepDuration failed: %v", err)
	}
}

func TestStepDuration_Failing(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	events := durationTestEvents("pkg/a", 10.0, 10.0)
	if err := store.Write(testRunStepID, "test-events.jsonl", []byte(events), h.Provenance{}); err != nil {
		t.Fatalf("store write: %v", err)
	}
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDuration(context.Background(), 5.0, testRunStepID)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, h.ErrDurationFailed) {
		t.Fatalf("expected ErrDurationFailed, got %v", err)
	}
	want := "duration failed: Tests with duration > 5.000s (required max):\n  10.000s  pkg/a.TestPass\n"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestStepDuration_ChecksAllPackages(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	events := durationTestEvents("pkg/slow", 99.0, 99.0) + "\n" + durationTestEvents("pkg/fast", 0.5, 0.5)
	if err := store.Write(testRunStepID, "test-events.jsonl", []byte(events), h.Provenance{}); err != nil {
		t.Fatalf("store write: %v", err)
	}
	harness, err := newTestHarness(testHarnessRoot, testPackages, validDeps(cmdtest.NewFakeRunner()), store, "")
	if err != nil {
		t.Fatalf("NewStepRunner failed: %v", err)
	}
	err = harness.StepDuration(context.Background(), 5.0, testRunStepID)
	if err == nil {
		t.Fatal("expected error: slow package must fail even when fast package passes")
	}
	if !errors.Is(err, h.ErrDurationFailed) {
		t.Fatalf("expected ErrDurationFailed, got %v", err)
	}
}
