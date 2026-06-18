// Vision: ArtifactStore implementations for cross-step file handoff; every StepRunner holds a non-nil store.
package harness

import "fmt"

// discardArtifactStore drops writes, errors on read—enough ArtifactStore for lint/build/deadcode/vet.
type discardArtifactStore struct{}

// NewDiscardArtifactStore returns an ArtifactStore that discards writes and errors on Read.
func NewDiscardArtifactStore() ArtifactStore {
	return discardArtifactStore{}
}

func (discardArtifactStore) Write(string, string, []byte, Provenance) error { return nil }

func (discardArtifactStore) Read(_, _ string) ([]byte, error) {
	return nil, fmt.Errorf("%w", ErrArtifactKeyMissing)
}

func (discardArtifactStore) Has(string, string) bool { return false }
