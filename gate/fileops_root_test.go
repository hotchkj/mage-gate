// Vision: Rooted production FileOps cannot re-bind gate root mid-instance.
package gate

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
)

func TestProductionFileOps_RootRejectRebindWithoutFilesystemUse(t *testing.T) {
	t.Parallel()
	fo := &productionFileOps{}
	const first = `/phase1-gate-fsops-root-a`
	const second = `/phase1-gate-fsops-root-b`
	if err := fo.Root(first); err != nil {
		t.Fatal(err)
	}
	err := fo.Root(second)
	if err == nil {
		t.Fatal("expected error on second distinct root without any IO method calls")
	}
	if !errors.Is(err, fileopspath.ErrFileOpsRootAlreadyBound) {
		t.Fatalf("got %v expected errors.Is(.., ErrFileOpsRootAlreadyBound)", err)
	}
}

func TestProductionFileOps_RootIdempotentSameCleaned(t *testing.T) {
	t.Parallel()
	fo := &productionFileOps{}
	const logical = `/phase1-idem-root`
	if err := fo.Root(logical); err != nil {
		t.Fatal(err)
	}
	trimmedVariant := `  ` + logical + `/.` + `  `
	want := filepath.Clean(strings.TrimSpace(trimmedVariant))
	if err := fo.Root(trimmedVariant); err != nil {
		t.Fatal(err)
	}
	// Third call proves idempotency is keyed on Clean(TrimSpace), not raw string equality.
	if err := fo.Root(want); err != nil {
		t.Fatalf("repeat clean root %q: %v", want, err)
	}
}
