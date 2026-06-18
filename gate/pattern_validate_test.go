// Vision: validateGoTestPackagePattern and NewPackageScope reject empty and invalid inputs at the package boundary.
package gate

import (
	"errors"
	"testing"
)

func TestValidateGoTestPackagePattern(t *testing.T) {
	t.Parallel()
	valid := "./..."
	if _, err := validateGoTestPackagePattern("", ErrPackageScopeEmpty); !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("empty: got %v", err)
	}
	if _, err := validateGoTestPackagePattern("-x", ErrPackageScopeEmpty); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("dash prefix: got %v", err)
	}
	if got, err := validateGoTestPackagePattern(valid, ErrPackageScopeEmpty); err != nil {
		t.Fatalf("valid: %v", err)
	} else if got != valid {
		t.Fatalf("valid: got %q, want %q", got, valid)
	}
	// TrimSpace: leading/trailing whitespace is stripped
	if got, err := validateGoTestPackagePattern("  ./...  ", ErrPackageScopeEmpty); err != nil {
		t.Fatalf("whitespace: %v", err)
	} else if got != valid {
		t.Fatalf("whitespace: got %q, want %q", got, valid)
	}
}

func TestNewPackageScopeEmpty(t *testing.T) {
	t.Parallel()
	_, err := NewPackageScope("")
	if !errors.Is(err, ErrPackageScopeEmpty) {
		t.Fatalf("NewPackageScope(\"\"): got %v, want ErrPackageScopeEmpty", err)
	}
}
