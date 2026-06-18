// Vision: Boundary condition tests for mutation kills logic previously surviving mutation testing.
package gatecheck

import (
	"errors"
	"testing"
)

func TestCheckMutationKillsRate_MinRateZero(t *testing.T) {
	t.Parallel()
	// Line 87-88: minRate <= 0 disables check
	check := &MutationKillsCheck{
		TotalKilled:     0,
		TotalLived:      10,
		KillRatePercent: 0.0,
	}
	err := CheckMutationKillsRate(check, 0)
	if err != nil {
		t.Fatalf("CheckMutationKillsRate(0) should pass, got: %v", err)
	}
	err = CheckMutationKillsRate(check, -1)
	if err != nil {
		t.Fatalf("CheckMutationKillsRate(-1) should pass, got: %v", err)
	}
}

func TestCheckMutationKillsRate_ZeroDenominator(t *testing.T) {
	t.Parallel()
	// Lines 91-93: denominator == 0 returns error when minRate > 0
	check := &MutationKillsCheck{
		TotalKilled: 0,
		TotalLived:  0,
	}
	err := CheckMutationKillsRate(check, 50)
	if err == nil {
		t.Fatal("expected error for zero denominator with positive minRate")
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestCheckMutationKillsRate_BelowThreshold(t *testing.T) {
	t.Parallel()
	// Lines 96-99: actual rate below minRate returns error
	check := &MutationKillsCheck{
		TotalKilled:     1,
		TotalLived:      1,
		KillRatePercent: 50.0,
	}
	err := CheckMutationKillsRate(check, 90)
	if err == nil {
		t.Fatal("expected error for kill rate below threshold")
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestCheckMutationCoverage_ZeroMinPercent(t *testing.T) {
	t.Parallel()
	// Line 109: minPercent <= 0 disables check
	check := &MutationKillsCheck{
		TotalNotCovered: 100,
	}
	err := CheckMutationCoverage(check, 0)
	if err != nil {
		t.Fatalf("CheckMutationCoverage(0) should pass, got: %v", err)
	}
}

func TestCheckMutationCoverage_ZeroTotal(t *testing.T) {
	t.Parallel()
	// Lines 114-115: total == 0 returns error
	check := &MutationKillsCheck{
		TotalKilled:     0,
		TotalLived:      0,
		TotalNotCovered: 0,
	}
	err := CheckMutationCoverage(check, 50)
	if err == nil {
		t.Fatal("expected error for zero total mutations")
	}
	if !errors.Is(err, ErrMutationCoverageFailed) {
		t.Fatalf("expected ErrMutationCoverageFailed, got %v", err)
	}
}

func TestEvaluateMutationKills_NilCheck(t *testing.T) {
	t.Parallel()
	// Lines 291-293: nil check returns error
	result := EvaluateMutationKills(nil, 90)
	if result.ThresholdError == nil {
		t.Fatal("expected ThresholdError for nil check")
	}
	if result.Passed {
		t.Fatal("expected Passed=false for nil check")
	}
}

func TestEvaluateMutationKills_LivedOverBudgetPositive(t *testing.T) {
	t.Parallel()
	// Lines 297-300: lived over budget calculation
	check := &MutationKillsCheck{
		TotalKilled: 90,
		TotalLived:  15, // 90 killed, 15 lived = 85.7% rate, at 90% threshold max allowed = 10
	}
	result := EvaluateMutationKills(check, 90)
	if result.LivedOverBudget <= 0 {
		t.Fatalf("expected positive LivedOverBudget, got %d", result.LivedOverBudget)
	}
}

func TestMutationKills_InvalidMinRateNegative(t *testing.T) {
	t.Parallel()
	// Lines 310-311: minRate < 0 returns error
	_, err := MutationKills([]byte(`{}`), -1)
	if err == nil {
		t.Fatal("expected error for negative minRate")
	}
	if !errors.Is(err, errInvalidMinKillRate) {
		t.Fatalf("expected errInvalidMinKillRate, got %v", err)
	}
}

func TestMutationKills_InvalidMinRateOver100(t *testing.T) {
	t.Parallel()
	// Lines 310-311: minRate > 100 returns error
	_, err := MutationKills([]byte(`{}`), 101)
	if err == nil {
		t.Fatal("expected error for minRate > 100")
	}
	if !errors.Is(err, errInvalidMinKillRate) {
		t.Fatalf("expected errInvalidMinKillRate, got %v", err)
	}
}
