// Vision: Human-readable mutation kill-rate summaries and per-file detail tables for gate output.
package gatecheck

import (
	"fmt"
	"sort"
	"strings"
)

// FormatMutationKillsReport formats the mutation kill rate check result for output.
// On pass: a short confirmation without anchoring on “how many killed”.
// On failure: states the gate miss, the required vs actual kill rate, a concrete lived budget
// (max lived allowed at the current killed count and how many are over), then timed out / not covered.
// On threshold failure, appends per-file tables for LIVED and/or TIMED_OUT when those totals are > 0.
func FormatMutationKillsReport(result MutationKillsResult) string {
	if result.Check == nil {
		return "mutationkills: no data"
	}
	check := result.Check
	var buf strings.Builder
	writeMutationKillsSummaryLine(&buf, result)
	if result.Passed {
		return buf.String()
	}
	writeMutationKillsDetailTables(&buf, check)
	return buf.String()
}

func writeMutationKillsDetailTables(buf *strings.Builder, check *MutationKillsCheck) {
	if check.TotalLived > 0 && len(mutationSurvivorsWithLived(check.Files)) > 0 {
		writeMutationSurvivorsTable(buf, check.Files)
	}
	if check.TotalTimedOut > 0 {
		writeMutationTimeoutsSection(buf, check.TotalTimedOut, check.Files)
	}
}

func writeMutationKillsSummaryLine(buf *strings.Builder, result MutationKillsResult) {
	check := result.Check
	if result.Passed {
		if result.MinKillRatePercent <= 0 {
			fmt.Fprintf(buf,
				"Mutation Kills: kill-rate threshold disabled (min 0%%). lived=%d timed_out=%d not_covered=%d",
				check.TotalLived, check.TotalTimedOut, check.TotalNotCovered,
			)
			return
		}
		fmt.Fprintf(buf,
			"Mutation Kills: kill-rate gate satisfied (required ≥%d%%). lived=%d timed_out=%d not_covered=%d",
			result.MinKillRatePercent, check.TotalLived, check.TotalTimedOut, check.TotalNotCovered,
		)
		return
	}

	// Failure path: lead with gate miss and remediation, not “success density”.
	den := check.TotalKilled + check.TotalLived
	if den == 0 {
		fmt.Fprintf(buf,
			"Mutation Kills: kill-rate gate not met - "+
				"no killed or lived mutants to score (denominator is 0). "+
				"timed_out=%d not_covered=%d",
			check.TotalTimedOut, check.TotalNotCovered,
		)
		return
	}
	if result.MinKillRatePercent <= 0 {
		fmt.Fprintf(buf,
			"Mutation Kills: kill-rate gate not met "+
				"(actual %.2f%%; threshold unset in report). "+
				"lived=%d timed_out=%d not_covered=%d",
			check.KillRatePercent, check.TotalLived, check.TotalTimedOut, check.TotalNotCovered,
		)
		return
	}

	minR := result.MinKillRatePercent
	lMax := maxLivedAllowedForMinKillRate(check.TotalKilled, minR)
	over := check.TotalLived - lMax
	if over < 0 {
		over = 0
	}
	fmt.Fprintf(buf,
		"Mutation Kills: kill-rate gate not met (required ≥%d%%, actual %.2f%%). "+
			"Remediation: strengthen tests so fewer mutants survive - "+
			"with %d killed mutants at most %d lived are allowed (have %d lived",
		minR, check.KillRatePercent, check.TotalKilled, lMax, check.TotalLived,
	)
	if over > 0 {
		fmt.Fprintf(buf, "; %d over that budget", over)
	}
	fmt.Fprintf(buf,
		"). timed_out=%d (hangs or slow paths under mutation). not_covered=%d.",
		check.TotalTimedOut, check.TotalNotCovered,
	)
}

func writeMutationSurvivorsTable(buf *strings.Builder, files []FileMutationStats) {
	survivors := mutationSurvivorsWithLived(files)
	sort.Slice(survivors, func(i, j int) bool {
		ai, aj := survivors[i], survivors[j]
		if ai.Lived != aj.Lived {
			return ai.Lived > aj.Lived
		}
		return ai.File < aj.File
	})
	buf.WriteString("\nSurvivors by file (LIVED > 0):\n")
	for _, f := range survivors {
		fmt.Fprintf(buf, "%4d  %s\n", f.Lived, f.File)
	}
}

func mutationSurvivorsWithLived(files []FileMutationStats) []FileMutationStats {
	survivors := make([]FileMutationStats, 0, len(files))
	for _, f := range files {
		if f.Lived > 0 {
			survivors = append(survivors, f)
		}
	}
	return survivors
}

func writeMutationTimeoutsSection(buf *strings.Builder, totalTimedOut int, files []FileMutationStats) {
	rows := mutationFilesWithTimedOut(files)
	if len(rows) == 0 {
		fmt.Fprintf(buf, "\nTimed out: %d (no per-file rows in report data)\n", totalTimedOut)
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := rows[i], rows[j]
		if ri.TimedOut != rj.TimedOut {
			return ri.TimedOut > rj.TimedOut
		}
		return ri.File < rj.File
	})
	buf.WriteString("\nTimed out by file (TIMED_OUT > 0):\n")
	for _, f := range rows {
		fmt.Fprintf(buf, "%4d  %s\n", f.TimedOut, f.File)
		for _, d := range f.TimedOutDetails {
			fmt.Fprintf(buf, "      - %s\n", d)
		}
	}
}

func mutationFilesWithTimedOut(files []FileMutationStats) []FileMutationStats {
	out := make([]FileMutationStats, 0, len(files))
	for _, f := range files {
		if f.TimedOut > 0 {
			out = append(out, f)
		}
	}
	return out
}

// FormatMutationKillsDetailSections builds survivor-by-file and timed-out sections (including per-mutation
// timeout detail lines) for silent-display ToolOutput without the summary line from FormatMutationKillsReport.
func FormatMutationKillsDetailSections(check *MutationKillsCheck) string {
	if check == nil {
		return ""
	}
	var buf strings.Builder
	writeMutationKillsDetailTables(&buf, check)
	return strings.TrimPrefix(buf.String(), "\n")
}
