package gatecheck

import "testing"

func TestFormatMutationKillsReport_Passed(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             true,
		MinKillRatePercent: 90,
		Check: &MutationKillsCheck{
			TotalKilled: 10, TotalLived: 0, TotalNotCovered: 1, TotalTimedOut: 0,
			KillRatePercent: 100.0,
		},
	})
	want := "Mutation Kills: kill-rate gate satisfied (required ≥90%). lived=0 timed_out=0 not_covered=1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_Passed_ThresholdDisabled(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             true,
		MinKillRatePercent: 0,
		Check: &MutationKillsCheck{
			TotalKilled: 3, TotalLived: 1, TotalNotCovered: 0, TotalTimedOut: 0,
			KillRatePercent: 75.0,
		},
	})
	want := "Mutation Kills: kill-rate threshold disabled (min 0%). lived=1 timed_out=0 not_covered=0"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedDenominatorZero_TimeoutsTableOnly(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 85,
		Check: &MutationKillsCheck{
			TotalKilled: 0, TotalLived: 0, TotalNotCovered: 2, TotalTimedOut: 1,
			KillRatePercent: 0,
			Files: []FileMutationStats{
				{File: "slow.go", TimedOut: 1},
			},
		},
	})
	want := "Mutation Kills: kill-rate gate not met - " +
		"no killed or lived mutants to score (denominator is 0). " +
		"timed_out=1 not_covered=2\n" +
		"Timed out by file (TIMED_OUT > 0):\n" +
		"   1  slow.go\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedWithLived_SurvivorTable(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 60,
		Check: &MutationKillsCheck{
			TotalKilled: 8, TotalLived: 7, TotalNotCovered: 0, TotalTimedOut: 0,
			KillRatePercent: 53.33,
			Files: []FileMutationStats{
				{File: "b.go", Lived: 5},
				{File: "a.go", Lived: 2},
			},
		},
	})
	want := "Mutation Kills: kill-rate gate not met (required ≥60%, actual 53.33%). " +
		"Remediation: strengthen tests so fewer mutants survive - " +
		"with 8 killed mutants at most 5 lived are allowed (have 7 lived; 2 over that budget). " +
		"timed_out=0 (hangs or slow paths under mutation). not_covered=0.\n" +
		"Survivors by file (LIVED > 0):\n" +
		"   5  b.go\n" +
		"   2  a.go\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedLivedAndTimedOutTables(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 90,
		Check: &MutationKillsCheck{
			TotalKilled: 1, TotalLived: 1, TotalNotCovered: 0, TotalTimedOut: 3,
			KillRatePercent: 50.0,
			Files: []FileMutationStats{
				{File: "only_timeout.go", Lived: 0, TimedOut: 3},
				{File: "has_lived.go", Lived: 1},
			},
		},
	})
	want := "Mutation Kills: kill-rate gate not met (required ≥90%, actual 50.00%). " +
		"Remediation: strengthen tests so fewer mutants survive - " +
		"with 1 killed mutants at most 0 lived are allowed (have 1 lived; 1 over that budget). " +
		"timed_out=3 (hangs or slow paths under mutation). not_covered=0.\n" +
		"Survivors by file (LIVED > 0):\n" +
		"   1  has_lived.go\n" +
		"\n" +
		"Timed out by file (TIMED_OUT > 0):\n" +
		"   3  only_timeout.go\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedSameFileLivedAndTimedOut(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 90,
		Check: &MutationKillsCheck{
			TotalKilled: 1, TotalLived: 2, TotalNotCovered: 0, TotalTimedOut: 1,
			KillRatePercent: 33.33,
			Files: []FileMutationStats{
				{File: "both.go", Lived: 2, TimedOut: 1},
			},
		},
	})
	want := "Mutation Kills: kill-rate gate not met (required ≥90%, actual 33.33%). " +
		"Remediation: strengthen tests so fewer mutants survive - " +
		"with 1 killed mutants at most 0 lived are allowed (have 2 lived; 2 over that budget). " +
		"timed_out=1 (hangs or slow paths under mutation). not_covered=0.\n" +
		"Survivors by file (LIVED > 0):\n" +
		"   2  both.go\n" +
		"\n" +
		"Timed out by file (TIMED_OUT > 0):\n" +
		"   1  both.go\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedTimeoutDetailLines(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 90,
		Check: &MutationKillsCheck{
			TotalKilled: 2, TotalLived: 1, TotalNotCovered: 0, TotalTimedOut: 1,
			KillRatePercent: 66.67,
			Files: []FileMutationStats{{
				File: "x.go", Lived: 1, TimedOut: 1,
				TimedOutDetails: []string{"CONDITIONALS_NEGATION line 3 col 9"},
			}},
		},
	})
	want := "Mutation Kills: kill-rate gate not met (required ≥90%, actual 66.67%). " +
		"Remediation: strengthen tests so fewer mutants survive - " +
		"with 2 killed mutants at most 0 lived are allowed (have 1 lived; 1 over that budget). " +
		"timed_out=1 (hangs or slow paths under mutation). not_covered=0.\n" +
		"Survivors by file (LIVED > 0):\n" +
		"   1  x.go\n" +
		"\n" +
		"Timed out by file (TIMED_OUT > 0):\n" +
		"   1  x.go\n" +
		"      - CONDITIONALS_NEGATION line 3 col 9\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsReport_FailedTimedOutAggregateOnlyNoFileRows(t *testing.T) {
	t.Parallel()
	got := FormatMutationKillsReport(MutationKillsResult{
		Passed:             false,
		MinKillRatePercent: 50,
		Check: &MutationKillsCheck{
			TotalKilled: 0, TotalLived: 4, TotalNotCovered: 0, TotalTimedOut: 2,
			KillRatePercent: 0,
			Files:           nil,
		},
	})
	want := "Mutation Kills: kill-rate gate not met (required ≥50%, actual 0.00%). " +
		"Remediation: strengthen tests so fewer mutants survive - " +
		"with 0 killed mutants at most 0 lived are allowed (have 4 lived; 4 over that budget). " +
		"timed_out=2 (hangs or slow paths under mutation). not_covered=0.\n" +
		"Timed out: 2 (no per-file rows in report data)\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatMutationKillsDetailSections_SurvivorsAndTimeouts(t *testing.T) {
	t.Parallel()
	check := &MutationKillsCheck{
		TotalLived:    1,
		TotalTimedOut: 1,
		Files: []FileMutationStats{
			{File: "x.go", Lived: 1, TimedOut: 1, TimedOutDetails: []string{"M line 1"}},
		},
	}
	got := FormatMutationKillsDetailSections(check)
	want := "Survivors by file (LIVED > 0):\n" +
		"   1  x.go\n" +
		"\n" +
		"Timed out by file (TIMED_OUT > 0):\n" +
		"   1  x.go\n" +
		"      - M line 1\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestMaxLivedAllowedForMinKillRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		killed, min int
		want        int
	}{
		{8, 60, 5},
		{10, 90, 1},
		{100, 90, 11},
		{1, 90, 0},
		{0, 90, 0},
		{8, 100, 0},
		{8, 0, 0},
	}
	for _, tc := range tests {
		got := maxLivedAllowedForMinKillRate(tc.killed, tc.min)
		if got != tc.want {
			t.Fatalf("maxLivedAllowedForMinKillRate(%d,%d) = %d, want %d", tc.killed, tc.min, got, tc.want)
		}
	}
}
