// Vision: Map harness/tool failures into DiagnosticError (silent display) vs raw sentinels (verbose display).
package gate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

// maxToolOutputBytes limits embedded tool text in silent-display diagnostics (verbose failures can blow context).
// Verbose display leaves errors uncapped.
const maxToolOutputBytes = 4096

// maxCrapOffendersInDiagnostic limits CRAP offender lines before summarizing the rest.
const (
	maxCrapOffendersInDiagnostic = 20
	crapSummaryLinesCap          = 2
)

// stepDiagnostic maps failures to Fix/Hint. Precedence: [ErrAllPackagesExcluded] first (even beside step sentinels),
// then step sentinels, else generic text. Existing *DiagnosticError keeps Fix/Hint; ToolOutput is UTF-8–capped then
// passed through filterFallbackToolOutput — line-scraping for scraped stderr-style blobs only
// (deadcode/test/crap/mutationsites).
// Structured threshold failures for coverage, mutationcoverage, and mutationkills build DiagnosticError upstream with
// curated ToolOutput; those step names intentionally fall through the filter (no extra line filtering here).
func stepDiagnostic(name string, err error) error {
	// Pass-through: keep Fix/Hint; set Cause to err (not de.Unwrap()) so errors.Is still sees outer wrappers.
	var de *DiagnosticError
	if errors.As(err, &de) {
		return cmdrunner.NewDiagnosticError(
			de.Name(),
			de.Message(),
			de.Fix(),
			de.Hint(),
			&cmdrunner.DiagnosticOptions{
				ToolOutput: truncateToolOutput(filterFallbackToolOutput(name, de.ToolOutput())),
				Cause:      err,
			},
		)
	}

	fix, hint := sentinelDiagnostic(name, err)
	return cmdrunner.NewDiagnosticError(
		name,
		fmt.Sprintf("%s failed", name),
		fix,
		hint,
		&cmdrunner.DiagnosticOptions{
			ToolOutput: truncateToolOutput(filterFallbackToolOutput(name, err.Error())),
			Cause:      err,
		},
	)
}

// filterFallbackToolOutput keeps high-signal lines from raw tool stderr / err.Error() text for steps that do not
// attach structured ToolOutput. Steps with dedicated silent renderers (coverage, mutationcoverage, mutationkills
// threshold paths, and others) use the default branch so curated output is unchanged.
func filterFallbackToolOutput(name, toolOutput string) string {
	switch name {
	case "deadcode":
		return filterDeadcodeOutput(toolOutput)
	case "markdownlint":
		return filterMarkdownlintOutput(toolOutput)
	case "test", "coveredtest":
		return filterTestOutput(toolOutput)
	case "crap":
		return filterCrapOutput(toolOutput)
	case "mutationsites":
		return filterMutationSitesScrapedOutput(toolOutput)
	case "mutationkills", "mutationscan", "mutationcoverage":
		return toolOutput
	default:
		return filterGenericStepOutput(toolOutput)
	}
}

func filterGenericStepOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	trimmedLines := make([]string, 0, len(lines))
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmedLines = append(trimmedLines, trimmed)
		if isHighSignalLine(trimmed) {
			kept = append(kept, trimmed)
		}
	}

	if len(kept) == 0 {
		return strings.Join(trimmedLines, "\n")
	}
	const maxLines = 8
	if len(kept) > maxLines {
		kept = kept[:maxLines]
	}
	return strings.Join(kept, "\n")
}

func isHighSignalLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "fail") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(trimmed, ".go:") ||
		strings.HasPrefix(trimmed, "# ")
}

func filterDeadcodeOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isDeadcodeSummaryLine(line) || isDeadcodeDetailLine(line) {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return toolOutput
	}
	return strings.Join(kept, "\n")
}

func isDeadcodeSummaryLine(line string) bool {
	return strings.Contains(line, "deadcode failed") || strings.Contains(line, "unreachable")
}

func isDeadcodeDetailLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, ".go:") {
		return true
	}
	return !strings.Contains(trimmed, " ") && strings.Count(trimmed, ".") >= 1
}

func filterMarkdownlintOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isMarkdownlintSummaryLine(line) || isMarkdownlintDetailLine(line) {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return toolOutput
	}
	return strings.Join(kept, "\n")
}

func isMarkdownlintSummaryLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "markdownlint failed") ||
		strings.Contains(lower, "markdown lint")
}

func isMarkdownlintDetailLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, ".md:") {
		return true
	}
	return strings.Contains(trimmed, ".go:") || isHighSignalLine(trimmed)
}

func filterTestOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isTestFailureSignalLine(line) || isTestFailureContextLine(line) {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return toolOutput
	}
	return strings.Join(kept, "\n")
}

func isTestFailureSignalLine(line string) bool {
	tokens := []string{
		`"Action":"fail"`,
		`"Output":"--- FAIL:`,
		`"Output":"FAIL\\t`,
		`"Output":"panic:`,
		"--- FAIL:",
		"FAIL\t",
		"panic:",
		"test failed:",
		"go test:",
	}
	for _, token := range tokens {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

func isTestFailureContextLine(line string) bool {
	if strings.Contains(line, `"Test":"`) {
		return false
	}
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "# ") ||
		strings.Contains(trimmed, ".go:") ||
		strings.HasPrefix(trimmed, "goroutine ") ||
		strings.Contains(trimmed, "runtime/")
}

func filterCrapOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	summary := make([]string, 0, crapSummaryLinesCap)
	offenders := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		collectCrapDiagnosticLines(line, &summary, &offenders)
	}
	if len(summary) == 0 && len(offenders) == 0 {
		return toolOutput
	}
	kept := append([]string{}, summary...)
	limit := maxCrapOffendersInDiagnostic
	if len(offenders) < limit {
		limit = len(offenders)
	}
	kept = append(kept, offenders[:limit]...)
	if len(offenders) > limit {
		kept = append(kept, fmt.Sprintf("  ... and %d more offender(s)", len(offenders)-limit))
	}
	return strings.Join(kept, "\n")
}

// filterMutationSitesScrapedOutput extracts high-signal lines from mutationsites failures that still rely on merged
// tool text. Lines that look like mutationcoverage summaries may appear in the same blob; kill-rate threshold copy
// is not targeted here (mutationkills uses structured ToolOutput, not this scraper).
func filterMutationSitesScrapedOutput(toolOutput string) string {
	lines := strings.Split(toolOutput, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isMutationDiagnosticLine(line) {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return toolOutput
	}
	return strings.Join(kept, "\n")
}

func isMutationDiagnosticLine(line string) bool {
	// mutationcoverage-style lines sometimes appear alongside mutationsites tool output
	// mutationsites: threshold / top-files / per-file site counts
	if isMutationCoverageHeaderLine(line) ||
		isMutationSiteFileLine(line) ||
		isMutationCoverageFileLine(line) ||
		isMutationTimeoutLine(line) {
		return true
	}
	return containsAnyMutationDiagnosticToken(line)
}

func containsAnyMutationDiagnosticToken(line string) bool {
	for _, token := range []string{
		"mutation coverage",
		"Timed out:",
		"mutation sites",
	} {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

func isMutationCoverageFileLine(line string) bool {
	// Match mutation coverage file lines: "  97.0%  path/to/file.go (X/Y not covered)"
	trimmed := strings.TrimSpace(line)
	// Pattern: percentage (one decimal) followed by file path and not covered count
	matched, _ := regexp.MatchString(`^\d+\.\d+%\s+.*\.go\s+\(\d+/\d+\s+not covered\)`, trimmed)
	return matched
}

func isMutationCoverageHeaderLine(line string) bool {
	// Match "Worst coverage files:" header
	return strings.TrimSpace(line) == "Worst coverage files:"
}

func isMutationTimeoutLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(line, "      - ") {
		return true
	}
	matched, _ := regexp.MatchString(`^Timed out: \d+`, trimmed)
	return matched
}

func isMutationSiteFileLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	// Match "  N  path/to/file.go" format (mutationsites)
	matched, _ := regexp.MatchString(`^\d+\s+.*\.go$`, trimmed)
	if matched {
		return true
	}
	// Match "pkg/file.go: N sites" format (mutationsites)
	matched, _ = regexp.MatchString(`.*\.go:\s*\d+\s+sites?`, trimmed)
	return matched
}

func collectCrapDiagnosticLines(line string, summary, offenders *[]string) {
	if isCrapSummaryLine(line) {
		*summary = append(*summary, line)
		return
	}
	if strings.Contains(line, " - CRAP=") {
		*offenders = append(*offenders, line)
	}
}

func isCrapSummaryLine(line string) bool {
	return strings.Contains(line, "crap failed:") || strings.Contains(line, "CRAP check failed:")
}

// sentinelDiagnostic matches step failure sentinels to Fix/Hint pairs (hints avoid mage-specific wording).
type sentinelMessage struct {
	sentinel error
	fix      string
	hint     string
}

func sentinelDiagnostic(name string, err error) (fix, hint string) {
	messages := []sentinelMessage{
		{
			sentinel: ErrAllPackagesExcluded,
			fix:      "widen the package scope or reduce exclusions",
			hint:     "the current exclude configuration filters out every package, making the check meaningless",
		},
		{
			sentinel: ErrLintFailed,
			fix:      "address the lint findings in the output below",
			hint:     "re-run the lint step in isolation to see full output",
		},
		{
			sentinel: ErrFormatFailed,
			fix:      "resolve the formatting issues reported below",
			hint:     "run the format step to apply fixes, then re-run lint",
		},
		{
			sentinel: ErrCompileFailed,
			fix:      "resolve the compilation error(s) in the output below",
			hint:     "check the file and line numbers in the error output",
		},
		{
			sentinel: ErrVetFailed,
			fix:      "resolve the vet finding(s) in the output below",
			hint:     "vet errors indicate code correctness issues, not style",
		},
		{
			sentinel: ErrTestFailed,
			fix:      "fix the failing test(s) identified in the output below",
			hint:     "run the failing test in verbose mode to see detailed output",
		},
		{
			sentinel: ErrCoverageFailed,
			fix:      "add tests to increase coverage to the required minimum",
			hint:     "use go tool cover -func on the coverage profile to find uncovered functions",
		},
		{
			sentinel: ErrMutationCoverageFailed,
			fix:      "expand the go test coverage profile (coverpkg) so more mutation points are considered covered",
			hint:     "NOT_COVERED mutants are outside the profile Gremlins used; improve line coverage in scoped packages",
		},
		{
			sentinel: ErrCrapFailed,
			fix:      "reduce complexity or increase test coverage for the listed functions",
			hint:     "CRAP = complexity^2 * (1 - coverage)^3 + complexity; either path reduces the score",
		},
		{
			sentinel: ErrDurationFailed,
			fix:      "optimize or split the slow tests listed below",
			hint:     "check for sleeps, network calls, expensive setup, or oversized grouped subtests in the slow tests",
		},
		{
			sentinel: ErrMutationSitesFailed,
			fix:      "reduce mutation site count by splitting large files",
			hint:     "rule of thumb: split by theme, move at least 30% of code out",
		},
		{
			sentinel: ErrDeadcodeFailed,
			fix:      "remove or reference the unreachable function(s)",
			hint:     "if the function is intentional public API, add it to the deadcode roots build tag",
		},
		{
			sentinel: ErrMarkdownLintFailed,
			fix:      "fix the markdown lint findings in the output below",
			hint:     "re-run the markdown lint step in isolation to see full output",
		},
		{
			sentinel: ErrMutationKillsFailed,
			fix:      "improve test coverage or add tests that kill surviving mutations",
			hint:     "focus on mutations marked as LIVED - these indicate gaps in test coverage",
		},
	}
	for _, msg := range messages {
		if errors.Is(err, msg.sentinel) {
			return msg.fix, msg.hint
		}
	}
	return fmt.Sprintf("review %s configuration", name),
		fmt.Sprintf("see %s output for details", name)
}

// truncateToolOutput UTF-8-safe cap at maxToolOutputBytes; suffix notes full length.
// Verbose display leaves tool logs uncapped.
func truncateToolOutput(toolOutput string) string {
	if len(toolOutput) <= maxToolOutputBytes {
		return toolOutput
	}
	cut := maxToolOutputBytes
	for cut > 0 && !utf8.ValidString(toolOutput[:cut]) {
		cut--
	}
	suffix := fmt.Sprintf(
		"\n... (truncated, %d bytes total — full output in verbose mode)",
		len(toolOutput),
	)
	return toolOutput[:cut] + suffix
}
