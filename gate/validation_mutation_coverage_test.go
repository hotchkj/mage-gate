// Vision: MutationCoverage threshold constructor and range validation.
package gate

import (
	"errors"
	"fmt"
	"testing"
)

func TestMinMutationCoverageConstructor(t *testing.T) {
	t.Parallel()
	threshold := MinMutationCoverage(85)
	if !threshold.set {
		t.Fatal("MinMutationCoverage should set the flag")
	}
}

func TestMinMutationCoverageValidateWhenNotSet(t *testing.T) {
	t.Parallel()
	err := validateMinMutationCoverage(MutationCoverageThreshold{})
	if err == nil {
		t.Fatal("expected error when MinMutationCoverage is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.Step() != "mutationcoverage" {
		t.Fatalf("expected step 'mutationcoverage', got %q", ve.Step())
	}
}

func TestMinMutationCoverageValidateZeroPercent(t *testing.T) {
	t.Parallel()
	threshold := MinMutationCoverage(0)
	err := validateMinMutationCoverage(threshold)
	if err != nil {
		t.Fatalf("MinMutationCoverage(0) should be valid (disables check), got %v", err)
	}
}

func TestMinMutationCoverageValidateValidRange(t *testing.T) {
	t.Parallel()
	testCases := []int{0, 1, 50, 99, 100}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			threshold := MinMutationCoverage(tc)
			err := validateMinMutationCoverage(threshold)
			if err != nil {
				t.Fatalf("MinMutationCoverage(%d) should be valid, got %v", tc, err)
			}
		})
	}
}

func TestMinMutationCoverageValidateNegativePercent(t *testing.T) {
	t.Parallel()
	threshold := MinMutationCoverage(-1)
	err := validateMinMutationCoverage(threshold)
	if err == nil {
		t.Fatal("expected error for negative percent")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMinMutationCoverageValidateAbove100Percent(t *testing.T) {
	t.Parallel()
	threshold := MinMutationCoverage(101)
	err := validateMinMutationCoverage(threshold)
	if err == nil {
		t.Fatal("expected error for percent > 100")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}
