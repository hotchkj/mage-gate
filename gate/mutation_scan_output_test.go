// Vision: [MutationScanOutput] is an opaque token; accessors enforce validity without field access.
package gate

import (
	"errors"
	"testing"
)

const internalPackagePattern = "./internal/..."

func TestMutationScanOutput_StepID(t *testing.T) {
	t.Parallel()
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		var o *MutationScanOutput
		_, err := o.StepID()
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("emptyStepID", func(t *testing.T) {
		t.Parallel()
		scope := mustNewQualityScope(t, "./...")
		o := &MutationScanOutput{qualityScope: scope}
		_, err := o.StepID()
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrMissingValue) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		scope := mustNewQualityScope(t, internalPackagePattern)
		o := &MutationScanOutput{stepID: "mutationscan-1", qualityScope: scope}
		id, err := o.StepID()
		if err != nil {
			t.Fatal(err)
		}
		if id != "mutationscan-1" {
			t.Fatalf("id = %q", id)
		}
	})
}

func TestMutationScanOutput_QualityScopeRejectsNil(t *testing.T) {
	t.Parallel()
	var o *MutationScanOutput
	_, err := o.QualityScope()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v", err)
	}
}

func TestMutationScanOutput_QualityScopeRejectsEmptyStepID(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, "./...")
	o := &MutationScanOutput{qualityScope: scope}
	_, err := o.QualityScope()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v", err)
	}
}

func TestMutationScanOutput_QualityScopeRejectsEmptyQualityScope(t *testing.T) {
	t.Parallel()
	o := &MutationScanOutput{stepID: "mutationscan-1"}
	_, err := o.QualityScope()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("got %v", err)
	}
}

func TestMutationScanOutput_QualityScopeReturnsScope(t *testing.T) {
	t.Parallel()
	scope := mustNewQualityScope(t, internalPackagePattern)
	o := &MutationScanOutput{stepID: "mutationscan-1", qualityScope: scope}
	qs, err := o.QualityScope()
	if err != nil {
		t.Fatal(err)
	}
	if qs.Packages() != internalPackagePattern {
		t.Fatalf("Packages = %q", qs.Packages())
	}
}
