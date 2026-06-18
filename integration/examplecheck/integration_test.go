//go:build integration

package examplecheck

// Vision: Curated fixtures proving gate steps fail with expected diagnostics when repos are intentionally broken.

// Integration justification: these tests require real tool execution (golangci-lint,
// deadcode, go test, go build) against intentionally-broken fixture repositories
// under testdata/failures/. Exact exit codes, diagnostic output formats, and
// tool-specific error behavior cannot be reproduced with in-process fakes.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const checkTimeout = 2 * time.Minute

const (
	lintToolSpec     = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
	deadcodeToolSpec = "golang.org/x/tools/cmd/deadcode@v0.31.0"
	gremlinsToolSpec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
)

var errDiagnosticOutputFormat = errors.New("expected ERROR/Fix/Hint diagnostic")

type stepCheck struct {
	name string
	run  func(*testing.T, context.Context, string)
}

// withChdir is process-global (os.Chdir). Tests using this function must not call t.Parallel().
func withChdir(dir string, fn func() error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir to %s: %w", dir, err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	return fn()
}

func captureStdoutStderr(fn func() error) (stdout, stderr string, runErr, err error) {
	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutReader, stdoutWriter, pipeErr := os.Pipe()
	if pipeErr != nil {
		return "", "", nil, pipeErr
	}
	stderrReader, stderrWriter, pipeErr := os.Pipe()
	if pipeErr != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return "", "", nil, pipeErr
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	type capture struct {
		data string
		err  error
	}
	stdoutCh := make(chan capture, 1)
	stderrCh := make(chan capture, 1)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		captureStdout, readErr := io.ReadAll(stdoutReader)
		stdoutCh <- capture{data: string(captureStdout), err: readErr}
	}()
	go func() {
		defer wg.Done()
		captureStderr, readErr := io.ReadAll(stderrReader)
		stderrCh <- capture{data: string(captureStderr), err: readErr}
	}()

	runErr = fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	wg.Wait()
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	stdoutCapture := <-stdoutCh
	stderrCapture := <-stderrCh
	if stdoutCapture.err != nil {
		return "", "", runErr, stdoutCapture.err
	}
	if stderrCapture.err != nil {
		return "", "", runErr, stderrCapture.err
	}
	return stdoutCapture.data, stderrCapture.data, runErr, nil
}

func goldenRoot(t *testing.T) string {
	t.Helper()
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve golden root: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filePath), "testdata", "golden", "agent-mode")
}

func writeGolden(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(goldenRoot(t), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write golden %s: %v", rel, err)
	}
}

func readGolden(t *testing.T, rel string) string {
	t.Helper()
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("invalid golden file path: %q", rel)
	}

	root := filepath.Clean(goldenRoot(t))
	path := filepath.Clean(filepath.Join(root, rel))
	if !strings.HasPrefix(path, root+string(filepath.Separator)) && path != root {
		t.Fatalf("golden path escapes root: %q", rel)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", rel, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

var (
	reLintPath = regexp.MustCompile(`run unit lint: .*golangci-lint(?:\.exe)?: exit status 1; `)
	// When ResolveToolCommand falls back to `go run`, subprocess failures surface as `go: exit status N`
	// inside harness prefixes — normalize to the stable tool name for golden comparison (not compile/test,
	// where `go:` is the real invoked binary).
	reToolResolveLintGoRun     = regexp.MustCompile(`run unit lint: go: exit status (\d+)`)
	reToolResolveDeadcodeGoRun = regexp.MustCompile(`deadcode command: go: exit status (\d+)`)
	reToolResolveGocycloGoRun  = regexp.MustCompile(`gocyclo report: go: exit status (\d+)`)
	reToolResolveGremlinsGoRun = regexp.MustCompile(`gremlins run: go: exit status (\d+)`)
	reLintFailLoc              = regexp.MustCompile(`lint_fail\.go:\d+:\d+:`)
	reToolExe                  = regexp.MustCompile(`\b([A-Za-z0-9_.-]+)\.exe\b`)
	reLeadingTime              = regexp.MustCompile(`^\{"Time":"[^"]*",`)
	reStdoutTime               = regexp.MustCompile(`stdout=\{"Time":"[^"]*",`)
	reElapsed                  = regexp.MustCompile(`"Elapsed":[0-9.]+`)
	reDuration                 = regexp.MustCompile(`"Duration":[0-9.]+`)
	reDurationMS               = regexp.MustCompile(`^(\s*)\d+\.\d+s\s+(.+)$`)
	reDrivePath                = regexp.MustCompile(`\b[A-Za-z]:\\[^\s"]*`)
	reDotRelPath               = regexp.MustCompile(`\.[\\/][^\s"]*`)
	reTempRoot                 = regexp.MustCompile(
		`(?:/(?:tmp|Temp|temp)(?:/[^\s"]+)+|\b[A-Za-z]:/[^\s"]*/(?:tmp|temp)/[^\s"]+)`,
	)
)

func normalizeAgentGolden(stepName, s string) string {
	if s == "" {
		return s
	}
	out := normalizeAgentGoldenTestOutput(s)
	switch stepName {
	case "test":
		out = normalizeAgentGoldenTestOutput(out)
	case "duration":
		out = normalizeAgentGoldenForDurationOutput(out)
	case "crap":
		out = normalizeAgentGoldenCrapOutput(out)
	case "lint":
		out = normalizeAgentGoldenLintOutput(out)
	}
	out = normalizeToolResolverGoRunExitAliases(out)
	out = strings.ReplaceAll(out, ".exe", "")
	out = normalizeTempRootPaths(out)
	return out
}

func normalizeToolResolverGoRunExitAliases(output string) string {
	output = reToolResolveLintGoRun.ReplaceAllString(output, "run unit lint: golangci-lint: exit status $1")
	output = reToolResolveDeadcodeGoRun.ReplaceAllString(output, "deadcode command: deadcode: exit status $1")
	output = reToolResolveGocycloGoRun.ReplaceAllString(output, "gocyclo report: gocyclo: exit status $1")
	output = reToolResolveGremlinsGoRun.ReplaceAllString(output, "gremlins run: gremlins: exit status $1")
	return output
}

func normalizeAgentGoldenTestOutput(output string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = normalizeBackslashPaths(output)
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		line = reLeadingTime.ReplaceAllString(line, "{")
		line = reStdoutTime.ReplaceAllString(line, "stdout={")
		line = reElapsed.ReplaceAllString(line, `"Elapsed":0`)
		line = reDuration.ReplaceAllString(line, `"Duration":0`)
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func normalizeAgentGoldenForDurationOutput(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if m := reDurationMS.FindStringSubmatch(line); len(m) == 3 {
			lines[i] = m[1] + m[2]
		}
	}
	return strings.Join(lines, "\n")
}

func normalizeAgentGoldenCrapOutput(output string) string {
	lines := strings.Split(output, "\n")
	crapLines, nonCrapLines := splitCrapGoldenLines(lines)
	sortedCrapLines := sortCrapLinesByScore(crapLines)
	keepCount := keepCrapLineCount(sortedCrapLines)
	return injectCrapSummaryIntoOutput(nonCrapLines, sortedCrapLines, keepCount)
}

func splitCrapGoldenLines(lines []string) (crapLines, nonCrapLines []string) {
	crapLines = make([]string, 0, len(lines))
	nonCrapLines = make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, " - CRAP=") {
			crapLines = append(crapLines, line)
		} else {
			nonCrapLines = append(nonCrapLines, line)
		}
	}
	return crapLines, nonCrapLines
}

func sortCrapLinesByScore(crapLines []string) []string {
	sort.SliceStable(crapLines, func(leftIndex, rightIndex int) bool {
		scoreI, scoreJ := parseCrapScore(crapLines[leftIndex]), parseCrapScore(crapLines[rightIndex])
		if scoreI == scoreJ {
			keyI := parseCrapSortKey(crapLines[leftIndex])
			keyJ := parseCrapSortKey(crapLines[rightIndex])
			if keyI == keyJ {
				return leftIndex < rightIndex
			}
			return keyI < keyJ
		}
		return scoreI > scoreJ
	})
	return crapLines
}

func keepCrapLineCount(crapLines []string) int {
	const maxKeepCrapLines = 3
	if len(crapLines) == 0 {
		return 0
	}
	if len(crapLines) <= maxKeepCrapLines {
		return len(crapLines)
	}
	return maxKeepCrapLines
}

func injectCrapSummaryIntoOutput(
	nonCrapLines, crapLines []string,
	keepCount int,
) string {
	rebuilt := make([]string, 0, len(nonCrapLines)+len(crapLines)+1)
	inserted := false
	for _, line := range nonCrapLines {
		if !inserted && strings.HasPrefix(line, "  ... and ") {
			rebuilt = append(rebuilt, fmt.Sprintf("  [CRAP offenders shown: %d]", len(crapLines)))
			rebuilt = append(rebuilt, crapLines[:keepCount]...)
			inserted = true
		}
		rebuilt = append(rebuilt, line)
	}
	if !inserted && len(crapLines) > 0 {
		rebuilt = append(rebuilt, fmt.Sprintf("  [CRAP offenders shown: %d]", len(crapLines)))
		rebuilt = append(rebuilt, crapLines[:keepCount]...)
	}
	return strings.Join(rebuilt, "\n")
}

func parseCrapScore(line string) float64 {
	before, after, found := strings.Cut(line, "CRAP=")
	if !found {
		return 0
	}
	_ = before
	parts := strings.Fields(after)
	if len(parts) == 0 {
		return 0
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return score
}

func parseCrapSortKey(line string) string {
	before, _, found := strings.Cut(line, " - CRAP=")
	if !found {
		return line
	}
	return strings.TrimSpace(before)
}

func normalizeAgentGoldenLintOutput(output string) string {
	output = reLintPath.ReplaceAllString(output, "run unit lint: golangci-lint: exit status 1; ")
	output = reToolExe.ReplaceAllString(output, "$1")
	// golangci-lint may emit different column offsets for the same finding across GOOS / toolchain builds.
	output = reLintFailLoc.ReplaceAllString(output, "lint_fail.go:9:13:")
	return output
}

func normalizeTempRootPaths(s string) string {
	return reTempRoot.ReplaceAllStringFunc(s, func(match string) string {
		trimmed := strings.TrimSuffix(match, "\"")
		tail := trimmed
		if idx := strings.LastIndexAny(trimmed, "/"); idx != -1 {
			tail = trimmed[idx+1:]
		}
		if tail == "" || tail == "." || strings.EqualFold(tail, "tmp") {
			return "/tmp"
		}
		return "/tmp/" + tail
	})
}

func normalizeBackslashPaths(output string) string {
	output = reDrivePath.ReplaceAllStringFunc(output, func(path string) string {
		return strings.ReplaceAll(path, "\\", "/")
	})

	output = reDotRelPath.ReplaceAllStringFunc(output, func(path string) string {
		return strings.ReplaceAll(path, "\\", "/")
	})

	return output
}

type normalizedGoldenCase struct {
	name     string
	stepName string
	input    string
	want     string
}

func normalizeAgentGoldenWindowsUnixVarianceCases() []normalizedGoldenCase {
	primary := normalizeAgentGoldenWindowsUnixVarianceCasesPrimary()
	resolverAliases := normalizeAgentGoldenWindowsUnixVarianceResolverAliasCases()
	return append(primary, resolverAliases...)
}

func normalizeAgentGoldenWindowsUnixVarianceCasesPrimary() []normalizedGoldenCase {
	return []normalizedGoldenCase{
		{
			name:     "test_json_and_backslashes",
			stepName: "test",
			input: `{"Time":"2026-05-07T11:00:00Z","Elapsed":12.345,"Duration":45.67}` + "\r\n" +
				`stdout={"Time":"2026-05-07T11:00:00Z","Elapsed":0.5,"Duration":1.2}` + "\r\n" +
				`C:\Users\me\repo\src\main.go`,
			want: `{"Elapsed":0,"Duration":0}` + "\n" +
				`stdout={"Elapsed":0,"Duration":0}` + "\n" +
				`C:/Users/me/repo/src/main.go`,
		},
		{
			name:     "lint_tool_name_and_exe",
			stepName: "lint",
			input:    `run unit lint: C:\bin\golangci-lint.exe: exit status 1; C:\bin\golangci-lint.exe --out-format=json`,
			want:     "run unit lint: golangci-lint: exit status 1; C:/bin/golangci-lint --out-format=json",
		},
		{
			name:     "lint_go_run_exit_when_resolver_skips_local_binary",
			stepName: "lint",
			input:    "lint failed: run unit lint: go: exit status 1; stdout=lint_fail.go:9:13: errcheck\n",
			want:     "lint failed: run unit lint: golangci-lint: exit status 1; stdout=lint_fail.go:9:13: errcheck\n",
		},
		{
			name:     "lint_fixture_location_columns_os_variance",
			stepName: "lint",
			input: "lint failed: run unit lint: golangci-lint: exit status 1; stdout=" +
				"lint_fail.go:9:99: Error return value of `fmt.Fprintf` is not checked (errcheck)\n" +
				`fmt.Fprintf(io.Discard, "lint failure fixture")`,
			want: "lint failed: run unit lint: golangci-lint: exit status 1; stdout=" +
				"lint_fail.go:9:13: Error return value of `fmt.Fprintf` is not checked (errcheck)\n" +
				`fmt.Fprintf(io.Discard, "lint failure fixture")`,
		},
		{
			name:     "duration_prefix",
			stepName: "duration",
			input:    "  1.234s  /Users/me/pkg/covered.go\n 0.500s /Users/me/pkg/timeout.go",
			want:     "  /Users/me/pkg/covered.go\n /Users/me/pkg/timeout.go",
		},
		{
			name:     "crap_row_sort_and_summary",
			stepName: "crap",
			input:    "  path.go\n  ... and 2 offender(s)\n  3.0 - CRAP=99.0\n  1.0 - CRAP=11.0\n  2.0 - CRAP=23.1",
			want: "  path.go\n  [CRAP offenders shown: 3]\n" +
				"  3.0 - CRAP=99.0\n  2.0 - CRAP=23.1\n" +
				"  1.0 - CRAP=11.0\n  ... and 2 offender(s)",
		},
		{
			name:     "crap_row_sort_tie_break_stable_by_path",
			stepName: "crap",
			input: "  path.go\n  ... and 2 offender(s)\n" +
				"  z.go:TinyFn01 - CRAP=2\n" +
				"  a.go:TinyFn02 - CRAP=2.0\n" +
				"  m.go:TinyFn03 - CRAP=2.0\n" +
				"  m.go:TinyFn00 - CRAP=11.0",
			want: "  path.go\n  [CRAP offenders shown: 4]\n" +
				"  m.go:TinyFn00 - CRAP=11.0\n  a.go:TinyFn02 - CRAP=2.0\n  m.go:TinyFn03 - CRAP=2.0\n" +
				"  ... and 2 offender(s)",
		},
		{
			name:     "unix_temp_root_is_normalized",
			stepName: "test",
			input:    `{"Time":"2026-05-07T11:00:00Z","Elapsed":9.87,"Duration":6.54}` + "\r\n" + `/tmp/ab12cd34/file.go`,
			want:     `{"Elapsed":0,"Duration":0}` + "\n" + `/tmp/file.go`,
		},
	}
}

func normalizeAgentGoldenWindowsUnixVarianceResolverAliasCases() []normalizedGoldenCase {
	return []normalizedGoldenCase{
		{
			name:     "deadcode_go_run_exit_when_resolver_skips_local_binary",
			stepName: "deadcode",
			input:    "deadcode failed: deadcode command: go: exit status 1\n",
			want:     "deadcode failed: deadcode command: deadcode: exit status 1\n",
		},
		{
			name:     "gocyclo_go_run_exit_when_resolver_skips_local_binary",
			stepName: "crap",
			input:    "crap failed: gocyclo report: go: exit status 1\n",
			want:     "crap failed: gocyclo report: gocyclo: exit status 1\n",
		},
		{
			name:     "gremlins_go_run_exit_when_resolver_skips_local_binary",
			stepName: "mutationsites",
			input:    "mutationsites failed: gremlins run: go: exit status 1\n",
			want:     "mutationsites failed: gremlins run: gremlins: exit status 1\n",
		},
	}
}

func TestNormalizeAgentGolden_CoversWindowsAndUnixVariance(t *testing.T) {
	t.Parallel()
	for _, tc := range normalizeAgentGoldenWindowsUnixVarianceCases() {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertNormalizedGoldenCase(t, &testCase)
		})
	}
}

func assertNormalizedGoldenCase(t *testing.T, tc *normalizedGoldenCase) {
	t.Helper()
	got := normalizeAgentGolden(tc.stepName, tc.input)
	if got != tc.want {
		t.Fatalf("%s mismatch.\nwant:\n%q\ngot:\n%q", tc.name, tc.want, got)
	}
}
