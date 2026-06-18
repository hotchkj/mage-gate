// Vision: MutationKills thresholds and output-token validation (illegal rates, empty reports, wiring mistakes).
package gate

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

func TestMinKillRateConstructor(t *testing.T) {
	t.Parallel()
	threshold := MinKillRate(85)
	if !threshold.set {
		t.Fatal("MinKillRate should set the flag")
	}
}

func TestMinKillRateValidateWhenNotSet(t *testing.T) {
	t.Parallel()
	err := validateMinKillRate(MinKillRateThreshold{})
	if err == nil {
		t.Fatal("expected error when MinKillRate is not set")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.Step() != "mutationkills" {
		t.Fatalf("expected step 'mutationkills', got %q", ve.Step())
	}
}

func TestMinKillRateValidateZeroPercent(t *testing.T) {
	t.Parallel()
	threshold := MinKillRate(0)
	err := validateMinKillRate(threshold)
	if err != nil {
		t.Fatalf("MinKillRate(0) should be valid (disables check), got %v", err)
	}
}

func TestMinKillRateValidateValidRange(t *testing.T) {
	t.Parallel()
	testCases := []int{0, 1, 50, 99, 100}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			threshold := MinKillRate(tc)
			err := validateMinKillRate(threshold)
			if err != nil {
				t.Fatalf("MinKillRate(%d) should be valid, got %v", tc, err)
			}
		})
	}
}

func TestMinKillRateValidateNegativePercent(t *testing.T) {
	t.Parallel()
	threshold := MinKillRate(-1)
	err := validateMinKillRate(threshold)
	if err == nil {
		t.Fatal("expected error for negative percent")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMinKillRateValidateAbove100Percent(t *testing.T) {
	t.Parallel()
	threshold := MinKillRate(101)
	err := validateMinKillRate(threshold)
	if err == nil {
		t.Fatal("expected error for percent > 100")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillsOutputInvalidTokenErrors(t *testing.T) {
	t.Parallel()
	out := MutationKillsOutput{}
	for name, fn := range map[string]func() (any, error){
		"TotalKilled":     func() (any, error) { return out.TotalKilled() },
		"TotalLived":      func() (any, error) { return out.TotalLived() },
		"KillRatePercent": func() (any, error) { return out.KillRatePercent() },
		"TotalNotCovered": func() (any, error) { return out.TotalNotCovered() },
		"TotalTimedOut":   func() (any, error) { return out.TotalTimedOut() },
		"TotalNotViable":  func() (any, error) { return out.TotalNotViable() },
		"TotalRunnable":   func() (any, error) { return out.TotalRunnable() },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := fn()
			if !errors.Is(err, ErrMissingValue) {
				t.Fatalf("expected ErrMissingValue, got %v", err)
			}
		})
	}
	var nilOut *MutationKillsOutput
	_, err := nilOut.TotalKilled()
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("nil receiver: expected ErrMissingValue, got %v", err)
	}
}

func TestMutationKillsCheckAccessors(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(out *MutationKillsOutput) (any, error)
		want any
	}{
		{"TotalKilled", func(out *MutationKillsOutput) (any, error) { return out.TotalKilled() }, int(0)},
		{"TotalLived", func(out *MutationKillsOutput) (any, error) { return out.TotalLived() }, int(0)},
		{"TotalNotCovered", func(out *MutationKillsOutput) (any, error) { return out.TotalNotCovered() }, int(0)},
		{"TotalTimedOut", func(out *MutationKillsOutput) (any, error) { return out.TotalTimedOut() }, int(0)},
		{"TotalNotViable", func(out *MutationKillsOutput) (any, error) { return out.TotalNotViable() }, int(0)},
		{"TotalRunnable", func(out *MutationKillsOutput) (any, error) { return out.TotalRunnable() }, int(0)},
		{"KillRatePercent", func(out *MutationKillsOutput) (any, error) { return out.KillRatePercent() }, float64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			check := &gatecheck.MutationKillsCheck{}
			out := MutationKillsOutput{stepID: "mk-test", check: check}
			got, err := tc.fn(&out)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestMutationKillRateMinZeroAlwaysPasses(t *testing.T) {
	t.Parallel()
	check := &gatecheck.MutationKillsCheck{TotalKilled: 0, TotalLived: 0, KillRatePercent: 0}
	out := MutationKillsOutput{stepID: "k", check: check}
	if err := MutationKillRate(out, MinKillRate(0)); err != nil {
		t.Fatalf("MinKillRate(0): %v", err)
	}
}

func TestMutationKillRateRejectsIncompleteOutput(t *testing.T) {
	t.Parallel()
	out := MutationKillsOutput{}
	if err := MutationKillRate(out, MinKillRate(0)); !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue, got %v", err)
	}
}

func TestMutationKillRateRejectsThresholdNotSet(t *testing.T) {
	t.Parallel()
	check := &gatecheck.MutationKillsCheck{TotalKilled: 5, TotalLived: 5, KillRatePercent: 50}
	out := MutationKillsOutput{stepID: "k", check: check}
	if err := MutationKillRate(out, MinKillRateThreshold{}); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("expected ErrInvalidOption, got %v", err)
	}
}

func TestMutationKillRateFailsBelowThreshold(t *testing.T) {
	t.Parallel()
	check := &gatecheck.MutationKillsCheck{TotalKilled: 1, TotalLived: 1, KillRatePercent: 50}
	out := MutationKillsOutput{stepID: "k", check: check}
	err := MutationKillRate(out, MinKillRate(90))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestMutationKillRateDenominatorZeroWithPositiveMin(t *testing.T) {
	t.Parallel()
	check := &gatecheck.MutationKillsCheck{
		TotalKilled: 0, TotalLived: 0, KillRatePercent: 0, TotalTimedOut: 2, TotalNotCovered: 1,
	}
	out := MutationKillsOutput{stepID: "k", check: check}
	err := MutationKillRate(out, MinKillRate(50))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMutationKillsFailed) {
		t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
	}
}

func TestMutationKillRateFailureUsesKillOutputModeForDiagnostics(t *testing.T) {
	t.Parallel()
	check := &gatecheck.MutationKillsCheck{
		TotalKilled:     8,
		TotalLived:      7,
		KillRatePercent: 53.33,
		TotalTimedOut:   1,
		TotalNotCovered: 0,
		Files: []gatecheck.FileMutationStats{
			{File: "pkg/b.go", Lived: 5},
			{File: "pkg/a.go", Lived: 2},
			{File: "pkg/t.go", TimedOut: 1, TimedOutDetails: []string{"CONDITIONALS_NEGATION line 3 col 9"}},
		},
	}
	t.Run("silent_token_wraps_diagnostic", func(t *testing.T) {
		t.Parallel()
		out := MutationKillsOutput{stepID: "k", check: check, outputMode: OutputModeAgent}
		err := MutationKillRate(out, MinKillRate(90))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if !errors.As(err, &de) {
			t.Fatalf("silent outputMode: want *DiagnosticError, got %T: %v", err, err)
		}
		if !errors.Is(err, ErrMutationKillsFailed) {
			t.Fatalf("errors.Is must still reach ErrMutationKillsFailed, got %v", err)
		}
		verifyMutationKillsSilentDiagnosticShape(t, de)
	})
	t.Run("zero_mode_stays_verbose_chain", func(t *testing.T) {
		t.Parallel()
		out := MutationKillsOutput{stepID: "k", check: check}
		err := MutationKillRate(out, MinKillRate(90))
		if err == nil {
			t.Fatal("expected error")
		}
		var de *DiagnosticError
		if errors.As(err, &de) {
			t.Fatalf("zero outputMode: expected raw verbose chain, got diagnostic %v", err)
		}
		if !errors.Is(err, ErrMutationKillsFailed) {
			t.Fatalf("expected ErrMutationKillsFailed, got %v", err)
		}
	})
}
