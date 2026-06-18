// Vision: CRAP inputs the harness should reject or normalize before scoring (bad coverage, missing cyclomatic data).
package gatecheck

import (
	"errors"
	"testing"
)

func TestCrap_InvalidMaxCrap(t *testing.T) {
	t.Parallel()
	_, err := Crap("", "", "github.com/hotchkj/mage-gate", "/repo", 0, nil)
	if err == nil {
		t.Fatal("expected error for zero maxCrap")
	}
	if !errors.Is(err, errCrapMaxNotPositive) {
		t.Fatalf("expected errCrapMaxNotPositive, got %v", err)
	}

	_, err = Crap("", "", "github.com/hotchkj/mage-gate", "/repo", -1, nil)
	if err == nil {
		t.Fatal("expected error for negative maxCrap")
	}
	if !errors.Is(err, errCrapMaxNotPositive) {
		t.Fatalf("expected errCrapMaxNotPositive, got %v", err)
	}
}
