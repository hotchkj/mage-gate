package gatetest_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/fileopspath"
)

func TestMemoryFileOps_RootIdempotentCleanEquivalent(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root(`/mem-root-stable`); err != nil {
		t.Fatalf("seed Root: %v", err)
	}
	for _, rootVariant := range []string{
		`/mem-root-stable`,
		`/mem-root-stable/`,
		`/mem-root-stable/.`,
		`/mem-root-stable/../mem-root-stable`,
	} {
		if err := mem.Root(rootVariant); err != nil {
			t.Fatalf("idempotent Root %q: %v", rootVariant, err)
		}
	}
	if err := mem.Root(`  /mem-root-stable  `); err != nil {
		t.Fatalf("TrimSpace envelope Root: %v", err)
	}
}

func TestMemoryFileOps_RootRejectRebindDifferent(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root(`/first-mem-root`); err != nil {
		t.Fatal(err)
	}
	err := mem.Root(`/second-mem-root`)
	if err == nil {
		t.Fatal("expected error on re-root")
	}
	if !errors.Is(err, fileopspath.ErrFileOpsRootAlreadyBound) {
		t.Fatalf("got %v expected errors.Is(.., ErrFileOpsRootAlreadyBound)", err)
	}
}

func TestMemoryFileOps_RootTrimSpaceEquivalence(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root(`/bind-once`); err != nil {
		t.Fatal(err)
	}
	if err := mem.Root(` /bind-once `); err != nil {
		t.Fatalf("Whitespace envelope after TrimSpace-clean path: %v", err)
	}
}
