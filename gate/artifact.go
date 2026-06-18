// Vision: Default in-process ArtifactStore—temp-backed files keyed by step ID for handoff between gate steps.
package gate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/hotchkj/mage-gate/internal/harness"
)

var (
	ErrNoArtifactForStep = errors.New("no artifacts for step")
	ErrArtifactNotFound  = errors.New("artifact not found for step")
	ErrArtifactSealed    = errors.New("artifact is sealed")
	ErrArtifactNotSealed = errors.New("artifact is not sealed")
)

// Provenance re-exports harness.Provenance for consumer access.
type Provenance = harness.Provenance

type storedArtifact struct {
	data       []byte
	provenance Provenance
	sealed     bool
}

// ArtifactStore maps step IDs to named, sealed blobs (insertion order preserved for searches).
type ArtifactStore struct {
	mu        sync.RWMutex
	artifacts map[string]map[string]storedArtifact
	stepOrder []string
}

func NewArtifactStore() *ArtifactStore {
	return &ArtifactStore{
		artifacts: make(map[string]map[string]storedArtifact),
	}
}

// ensureStepLocked creates the per-step map on first use and appends stepID
// to stepOrder so FindArtifact iteration matches write order.
func (s *ArtifactStore) ensureStepLocked(stepID string) {
	if s.artifacts[stepID] != nil {
		return
	}
	s.artifacts[stepID] = make(map[string]storedArtifact)
	s.stepOrder = append(s.stepOrder, stepID)
}

// Write stores and seals name under stepID (second write returns [ErrArtifactSealed]).
func (s *ArtifactStore) Write(stepID, name string, data []byte, prov Provenance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureStepLocked(stepID)
	if _, exists := s.artifacts[stepID][name]; exists {
		return fmt.Errorf("%w: %s/%s", ErrArtifactSealed, stepID, name)
	}
	s.artifacts[stepID][name] = storedArtifact{
		data:       bytes.Clone(data),
		provenance: prov,
		sealed:     true,
	}
	return nil
}

// Writer streams bytes into an artifact until Close seals it.
func (s *ArtifactStore) Writer(stepID, name string, prov Provenance) io.WriteCloser {
	return &artifactWriter{store: s, stepID: stepID, name: name, prov: prov}
}

func (s *ArtifactStore) Read(stepID, name string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stepArtifacts, ok := s.artifacts[stepID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoArtifactForStep, stepID)
	}
	sa, ok := stepArtifacts[name]
	if !ok {
		return nil, fmt.Errorf("%w %q: %q", ErrArtifactNotFound, stepID, name)
	}
	if !sa.sealed {
		return nil, fmt.Errorf("%w: %s/%s", ErrArtifactNotSealed, stepID, name)
	}
	return bytes.Clone(sa.data), nil
}

func (s *ArtifactStore) ReadProvenance(stepID, name string) (Provenance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stepArtifacts, ok := s.artifacts[stepID]
	if !ok {
		return Provenance{}, fmt.Errorf("%w: %q", ErrNoArtifactForStep, stepID)
	}
	sa, ok := stepArtifacts[name]
	if !ok {
		return Provenance{}, fmt.Errorf("%w %q: %q", ErrArtifactNotFound, stepID, name)
	}
	if !sa.sealed {
		return Provenance{}, fmt.Errorf("%w: %s/%s", ErrArtifactNotSealed, stepID, name)
	}
	return sa.provenance, nil
}

// FindArtifact returns the earliest stepID (by creation order) that holds a sealed name.
func (s *ArtifactStore) FindArtifact(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, stepID := range s.stepOrder {
		artifacts := s.artifacts[stepID]
		if sa, ok := artifacts[name]; ok && sa.sealed {
			return stepID, true
		}
	}

	return "", false
}

// FindArtifactByStepPrefix matches step IDs with prefix+"-" before searching for name.
func (s *ArtifactStore) FindArtifactByStepPrefix(
	prefix, name string,
) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pfx := prefix + "-"
	for _, stepID := range s.stepOrder {
		if len(stepID) < len(pfx) || stepID[:len(pfx)] != pfx {
			continue
		}
		artifacts := s.artifacts[stepID]
		if sa, ok := artifacts[name]; ok && sa.sealed {
			return stepID, true
		}
	}

	return "", false
}

func (s *ArtifactStore) Reader(stepID, name string) (io.Reader, error) {
	data, err := s.Read(stepID, name)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (s *ArtifactStore) Has(stepID, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stepArtifacts, ok := s.artifacts[stepID]
	if !ok {
		return false
	}
	_, ok = stepArtifacts[name]
	return ok
}

type artifactWriter struct {
	store  *ArtifactStore
	stepID string
	name   string
	prov   Provenance
	closed bool
}

func (w *artifactWriter) Write(data []byte) (n int, err error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if w.closed {
		return 0, fmt.Errorf("%w: writer closed for %s/%s", ErrArtifactSealed, w.stepID, w.name)
	}
	w.store.ensureStepLocked(w.stepID)
	stepMap := w.store.artifacts[w.stepID]
	if sa, ok := stepMap[w.name]; ok && sa.sealed {
		return 0, fmt.Errorf("%w: %s/%s", ErrArtifactSealed, w.stepID, w.name)
	}
	existing := stepMap[w.name]
	stepMap[w.name] = storedArtifact{
		data:       append(existing.data, data...),
		provenance: w.prov,
		sealed:     false,
	}
	return len(data), nil
}

func (w *artifactWriter) Close() error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if w.closed {
		return fmt.Errorf("%w: %s/%s", ErrArtifactSealed, w.stepID, w.name)
	}
	w.closed = true
	w.store.ensureStepLocked(w.stepID)
	stepMap := w.store.artifacts[w.stepID]
	sa, exists := stepMap[w.name]
	if !exists {
		stepMap[w.name] = storedArtifact{
			data:       nil,
			provenance: w.prov,
			sealed:     true,
		}
		return nil
	}
	sa.sealed = true
	stepMap[w.name] = sa
	return nil
}
