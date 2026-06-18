// Vision: Mutation wiring guardrails—artifact store usage requires a step ID when mutation capture is enabled.
package harness

import (
	"errors"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestStoreMutationArtifactEmptyStepID(t *testing.T) {
	t.Parallel()
	harn, err := NewStepRunner(
		"/test-root",
		"art",
		"./...",
		cmdtest.NewFakeRunner(),
		gatetest.NewMemoryFileOps(),
		NewDiscardArtifactStore(),
		"",
		WithToolResolver(gatetest.NewFakeToolResolver().SetDefaultToLocal(true)),
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	err = harn.storeMutationArtifact("art/mutations.json")
	if err == nil {
		t.Fatal("expected error when step ID is empty")
	}
	if !errors.Is(err, ErrMutationSitesFailed) {
		t.Fatalf("expected ErrMutationSitesFailed, got %v", err)
	}
}
