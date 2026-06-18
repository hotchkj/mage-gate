// Vision: Store implementations honor discard/retain: temp cleanup, sealed steps, and error semantics per contract.
package harness_test

import (
	"errors"
	"testing"

	h "github.com/hotchkj/mage-gate/internal/harness"
)

func TestDiscardArtifactStore(t *testing.T) {
	t.Parallel()
	store := h.NewDiscardArtifactStore()
	const (
		stepKey = "s"
		nameKey = "n"
	)
	if err := store.Write(stepKey, nameKey, []byte("data"), h.Provenance{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_, err := store.Read(stepKey, nameKey)
	if err == nil {
		t.Fatal("expected error from discard store Read")
	}
	if !errors.Is(err, h.ErrArtifactKeyMissing) {
		t.Fatalf("expected ErrArtifactKeyMissing, got %v", err)
	}
	if store.Has(stepKey, nameKey) {
		t.Fatal("expected Has to return false")
	}
}
