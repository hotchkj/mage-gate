// Vision: DefaultArtifactStore contracts: read/write/seal, temp roots, and provenance on the public gate API.
package gate

import (
	"bytes"
	"errors"
	"testing"
)

func TestArtifactStoreWriteRead(t *testing.T) {
	store := NewArtifactStore()

	data := []byte("test data")
	err := store.Write("step-1", "coverage.out", data, Provenance{})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := store.Read("step-1", "coverage.out")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("expected %q, got %q", string(data), string(got))
	}
}

func TestArtifactStoreReadProvenance(t *testing.T) {
	store := NewArtifactStore()
	want := Provenance{StepID: "step-1", Tool: "go test", Packages: "./..."}
	err := store.Write("step-1", "coverage.out", []byte("x"), want)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	got, err := store.ReadProvenance("step-1", "coverage.out")
	if err != nil {
		t.Fatalf("ReadProvenance: %v", err)
	}
	if got != want {
		t.Fatalf("provenance mismatch: got %+v want %+v", got, want)
	}
}

func TestArtifactStoreMissingStep(t *testing.T) {
	store := NewArtifactStore()

	_, err := store.Read("missing-step", "coverage.out")
	if err == nil {
		t.Fatal("expected error for missing step")
	}
	if !errors.Is(err, ErrNoArtifactForStep) {
		t.Fatalf("expected ErrNoArtifactForStep, got %v", err)
	}
}

func TestArtifactStoreMissingArtifact(t *testing.T) {
	store := NewArtifactStore()

	err := store.Write("step-1", "coverage.out", []byte("data"), Provenance{})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, err = store.Read("step-1", "missing-artifact")
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("expected ErrArtifactNotFound, got %v", err)
	}
}

func TestArtifactStoreHas(t *testing.T) {
	store := NewArtifactStore()

	if store.Has("step-1", "coverage.out") {
		t.Fatal("expected Has to return false for non-existent artifact")
	}

	err := store.Write("step-1", "coverage.out", []byte("data"), Provenance{})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if !store.Has("step-1", "coverage.out") {
		t.Fatal("expected Has to return true for existing artifact")
	}
}

func TestArtifactStoreWriter(t *testing.T) {
	store := NewArtifactStore()

	want := Provenance{StepID: "step-1", Tool: "go test -json", Packages: "./..."}
	writer := store.Writer("step-1", "test-events.jsonl", want)
	data := []byte(`{"Action":"pass","Package":"pkg"}`)
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes written, got %d", len(data), n)
	}
	closeErr := writer.Close()
	if closeErr != nil {
		t.Fatalf("close failed: %v", closeErr)
	}

	got, err := store.Read("step-1", "test-events.jsonl")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("expected %q, got %q", string(data), string(got))
	}
	prov, err := store.ReadProvenance("step-1", "test-events.jsonl")
	if err != nil {
		t.Fatalf("ReadProvenance: %v", err)
	}
	if prov != want {
		t.Fatalf("expected provenance %+v, got %+v", want, prov)
	}
}

func TestArtifactStoreReader(t *testing.T) {
	store := NewArtifactStore()

	data := []byte("test data")
	err := store.Write("step-1", "coverage.out", data, Provenance{})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	reader, err := store.Reader("step-1", "coverage.out")
	if err != nil {
		t.Fatalf("reader failed: %v", err)
	}

	buf := make([]byte, len(data))
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(buf[:n], data) {
		t.Fatalf("expected %q, got %q", string(data), string(buf[:n]))
	}
}

func TestArtifactStoreReaderMissing(t *testing.T) {
	store := NewArtifactStore()

	_, err := store.Reader("missing-step", "coverage.out")
	if err == nil {
		t.Fatal("expected error for missing step")
	}
	if !errors.Is(err, ErrNoArtifactForStep) {
		t.Fatalf("expected ErrNoArtifactForStep, got %v", err)
	}
}

func TestArtifactStoreFindArtifact(t *testing.T) {
	t.Parallel()
	store := NewArtifactStore()
	if err := store.Write("step-first", "coverage.out", []byte("first"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}
	if err := store.Write("step-second", "coverage.out", []byte("second"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}
	if err := store.Write("step-first", "test-events.jsonl", []byte("y"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}

	stepID, found := store.FindArtifact("coverage.out")
	if !found {
		t.Fatal("expected to find coverage.out")
	}
	if stepID != "step-first" {
		t.Fatalf("FindArtifact must return first-written step; got %q, want step-first", stepID)
	}

	_, found = store.FindArtifact("missing.txt")
	if found {
		t.Fatal("expected not to find missing.txt")
	}
}

func TestArtifactStoreFindByStepPrefix(t *testing.T) {
	t.Parallel()
	store := NewArtifactStore()
	if err := store.Write("coverage-first", "coverage.out", []byte("first"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}
	if err := store.Write("coverage-second", "coverage.out", []byte("second"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}
	if err := store.Write("test-1", "coverage.out", []byte("other"), Provenance{}); err != nil {
		t.Fatalf("store.Write: %v", err)
	}

	stepID, found := store.FindArtifactByStepPrefix("coverage", "coverage.out")
	if !found {
		t.Fatal("expected to find coverage.out under coverage-*")
	}
	if stepID != "coverage-first" {
		t.Fatalf(
			"FindArtifactByStepPrefix must return first step in write order; got %q, want coverage-first",
			stepID,
		)
	}

	_, found = store.FindArtifactByStepPrefix("crap", "coverage.out")
	if found {
		t.Fatal("expected not to find coverage.out under crap-*")
	}
}

func TestArtifactStoreWriteCopiesInput(t *testing.T) {
	store := NewArtifactStore()
	data := []byte("original")
	if err := store.Write("s", "a", data, Provenance{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	data[0] = 'X'
	got, err := store.Read("s", "a")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("store mutated by caller slice: got %q", got)
	}
}

func TestArtifactStoreReadReturnsIndependentCopy(t *testing.T) {
	store := NewArtifactStore()
	if err := store.Write("s", "a", []byte("abc"), Provenance{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	first, err := store.Read("s", "a")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	first[0] = 'z'
	second, err := store.Read("s", "a")
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if string(second) != "abc" {
		t.Fatalf("second read after mutating first: got %q", second)
	}
}

func TestArtifactStoreWriteToSealedKeyFails(t *testing.T) {
	store := NewArtifactStore()
	if err := store.Write("s", "a", []byte("one"), Provenance{}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	err := store.Write("s", "a", []byte("two"), Provenance{})
	if err == nil {
		t.Fatal("expected error on second write to same key")
	}
	if !errors.Is(err, ErrArtifactSealed) {
		t.Fatalf("expected ErrArtifactSealed, got %v", err)
	}
}

func TestArtifactStoreWriterSealsOnClose(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("s", "a", Provenance{})
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, werr := writer.Write([]byte("y"))
	if werr == nil {
		t.Fatal("expected error writing after close")
	}
	if !errors.Is(werr, ErrArtifactSealed) {
		t.Fatalf("expected ErrArtifactSealed, got %v", werr)
	}
}

func TestArtifactStoreReadUnsealedFails(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("s", "a", Provenance{})
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := store.Read("s", "a")
	if err == nil {
		t.Fatal("expected error before Close")
	}
	if !errors.Is(err, ErrArtifactNotSealed) {
		t.Fatalf("expected ErrArtifactNotSealed, got %v", err)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
}

func TestArtifactStoreHasReturnsTrueWhileUnsealed(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("s", "a", Provenance{})
	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !store.Has("s", "a") {
		t.Fatal("expected Has to return true for unsealed (mid-stream) artifact")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !store.Has("s", "a") {
		t.Fatal("expected Has to return true for sealed artifact")
	}
}

func TestArtifactStoreWriterDoubleCloseFails(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("s", "a", Provenance{})
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	err := writer.Close()
	if err == nil {
		t.Fatal("expected error on second Close")
	}
	if !errors.Is(err, ErrArtifactSealed) {
		t.Fatalf("expected ErrArtifactSealed, got %v", err)
	}
}

func TestArtifactStoreFindArtifactSkipsUnsealed(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("step-1", "coverage.out", Provenance{})
	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, found := store.FindArtifact("coverage.out"); found {
		t.Fatal("expected unsealed artifact to be invisible to FindArtifact")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	stepID, found := store.FindArtifact("coverage.out")
	if !found || stepID != "step-1" {
		t.Fatalf("expected sealed artifact found as step-1, found=%v stepID=%q", found, stepID)
	}
}

func TestArtifactStoreReadProvenanceUnsealedFails(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("s", "a", Provenance{StepID: "s", Tool: "test"})
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := store.ReadProvenance("s", "a")
	if err == nil {
		t.Fatal("expected error reading provenance before Close")
	}
	if !errors.Is(err, ErrArtifactNotSealed) {
		t.Fatalf("expected ErrArtifactNotSealed, got %v", err)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
	prov, err := store.ReadProvenance("s", "a")
	if err != nil {
		t.Fatalf("ReadProvenance after seal: %v", err)
	}
	if prov.StepID != "s" {
		t.Fatalf("expected StepID %q, got %q", "s", prov.StepID)
	}
}

func TestArtifactStoreFindByStepPrefixSkipsUnsealed(t *testing.T) {
	store := NewArtifactStore()
	writer := store.Writer("test-1", "coverage.out", Provenance{})
	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, found := store.FindArtifactByStepPrefix("test", "coverage.out"); found {
		t.Fatal("expected unsealed artifact to be invisible to FindArtifactByStepPrefix")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	stepID, found := store.FindArtifactByStepPrefix("test", "coverage.out")
	if !found || stepID != "test-1" {
		t.Fatalf("expected sealed artifact at test-1, found=%v stepID=%q", found, stepID)
	}
}
