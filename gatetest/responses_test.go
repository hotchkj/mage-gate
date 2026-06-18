// Vision: Golden-style checks that response factories emit argv/stderr shapes the harness steps parse.
package gatetest_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestNoopCommand_Succeeds(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "vet", "./...")
	var stdout, stderr strings.Builder
	err := gatetest.NoopCommand(context.Background(), cmd, &stdout, &stderr)
	if err != nil {
		t.Fatalf("NoopCommand returned error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("NoopCommand wrote to stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("NoopCommand wrote to stderr: %q", stderr.String())
	}
}

var errTestSentinel = errors.New("test error")

func TestFail_ReturnsError(t *testing.T) {
	t.Parallel()
	response := gatetest.Fail(errTestSentinel)
	cmd := cmdrunner.NewCommand(".", "go", "test")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if !errors.Is(err, errTestSentinel) {
		t.Fatalf("Fail returned %v, want %v", err, errTestSentinel)
	}
}

func TestGoTestPass_EmitsJSON(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	response := gatetest.GoTestPass(mem, "example.com/mod/pkg")
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...")
	var stdout strings.Builder
	err := response(context.Background(), cmd, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPass error: %v", err)
	}
	out := stdout.String()
	// Response emits: run event, test-level pass event, package-level pass event.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSON lines (run, test-pass, pkg-pass), got %d: %q", len(lines), out)
	}

	type jsonEvent struct {
		Action string `json:"Action"`
		Test   string `json:"Test"`
	}

	var ev0 jsonEvent
	if err := json.Unmarshal([]byte(lines[0]), &ev0); err != nil {
		t.Fatalf("lines[0] is not valid JSON: %v", err)
	}
	if ev0.Action != "run" {
		t.Errorf("lines[0].Action = %q, want \"run\"", ev0.Action)
	}

	var ev2 jsonEvent
	if err := json.Unmarshal([]byte(lines[2]), &ev2); err != nil {
		t.Fatalf("lines[2] is not valid JSON: %v", err)
	}
	if ev2.Action != "pass" {
		t.Errorf("lines[2].Action = %q, want \"pass\"", ev2.Action)
	}
}

func TestGoTestPass_WritesCoverprofile(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	response := gatetest.GoTestPass(mem, "pkg")
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile=artifacts/coverage.out", "./...")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPass error: %v", err)
	}
	data, readErr := mem.ReadFile("artifacts/coverage.out")
	if readErr != nil {
		t.Fatalf("coverprofile not written: %v", readErr)
	}
	assertMinimalCoverprofile(t, data)
}

func TestGoTestPass_WritesCoverprofile_TwoToken(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	response := gatetest.GoTestPass(mem, "pkg")
	// Space-separated form: "-coverprofile" "artifacts/coverage.out" — as real go test emits.
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile", "artifacts/coverage.out", "./...")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPass error: %v", err)
	}
	data, readErr := mem.ReadFile("artifacts/coverage.out")
	if readErr != nil {
		t.Fatalf("coverprofile not written for two-token form: %v", readErr)
	}
	assertMinimalCoverprofile(t, data)
}

func assertMinimalCoverprofile(tb testing.TB, data []byte) {
	tb.Helper()
	body := string(data)
	if !strings.HasPrefix(body, "mode: set\n") {
		tb.Fatalf("coverprofile header = %q, want mode: set", body)
	}
	if body == "mode: set\n" {
		tb.Fatal("coverprofile contains only a mode header")
	}
}

func TestGoTestPass_NoCoverprofFlag(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	response := gatetest.GoTestPass(mem, "pkg")
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPass error: %v", err)
	}
}

func TestGoTestPassWithCoverage_WritesCoverprofile_TwoToken(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	response := gatetest.GoTestPassWithCoverage(mem, "pkg", 50)
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile", "artifacts/coverage.out", "./...")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPassWithCoverage: %v", err)
	}
	data, readErr := mem.ReadFile("artifacts/coverage.out")
	if readErr != nil {
		t.Fatalf("coverprofile read: %v", readErr)
	}
	if !strings.HasPrefix(string(data), "mode: set\n") {
		t.Fatalf("profile header: %q", string(data))
	}
}

func TestGoTestPassWithCoverage_ZeroWritesAllStatementsUncovered(t *testing.T) {
	t.Parallel()

	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	response := gatetest.GoTestPassWithCoverage(mem, "pkg", 0)
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile=artifacts/coverage.out", "./...")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("GoTestPassWithCoverage: %v", err)
	}
	data, readErr := mem.ReadFile("artifacts/coverage.out")
	if readErr != nil {
		t.Fatalf("coverprofile read: %v", readErr)
	}
	want := "mode: set\nexample.com/mod/pkg/file.go:3.1,4.2 100 0\n"
	if string(data) != want {
		t.Fatalf("profile = %q, want %q", string(data), want)
	}
}

func TestGoTestPassWithCoverage_OutOfRangeErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pct  int
	}{
		{name: "negative", pct: -3},
		{name: "over100", pct: 150},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mem := gatetest.NewMemoryFileOps()
			response := gatetest.GoTestPassWithCoverage(mem, "pkg", tc.pct)
			cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile=artifacts/coverage.out", "./...")
			err := response(context.Background(), cmd, io.Discard, io.Discard)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, gatetest.ErrFakeCoveragePercentOutOfRange) {
				t.Fatalf("errors.Is: got %v", err)
			}
			if _, readErr := mem.ReadFile("artifacts/coverage.out"); readErr == nil {
				t.Fatal("expected no coverprofile")
			}
		})
	}
}

func TestGoToolCover_OutputsTotal(t *testing.T) {
	t.Parallel()
	response := gatetest.GoToolCover(95.0)
	cmd := cmdrunner.NewCommand(".", "go", "tool", "cover", "-func=cov.out")
	var stdout strings.Builder
	err := response(context.Background(), cmd, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("GoToolCover error: %v", err)
	}
	want := "total:\t(statements)\t95.0%\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestGoToolCoverFunc_OutputsFuncsAndTotal(t *testing.T) {
	t.Parallel()
	funcs := map[string]float64{
		"pkg/foo.go:10:\tValidate": 100.0,
	}
	response := gatetest.GoToolCoverFunc(funcs, 95.0)
	cmd := cmdrunner.NewCommand(".", "go", "tool", "cover", "-func=cov.out")
	var stdout strings.Builder
	err := response(context.Background(), cmd, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("GoToolCoverFunc error: %v", err)
	}
	wantOut := "pkg/foo.go:10:\tValidate\t\t100.0%\ntotal:\t(statements)\t95.0%\n"
	if stdout.String() != wantOut {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantOut)
	}
}

func TestGoList(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		args       []string
		wantOutput string
	}{
		{
			name:       "ModulePath",
			args:       []string{"go", "list", "-m", "-f", "{{.Path}}"},
			wantOutput: "example.com/mod",
		},
		{
			name:       "ModuleDir",
			args:       []string{"go", "list", "-m", "-f", "{{.Dir}}"},
			wantOutput: "/root/mod",
		},
		{
			name:       "PackageImportPaths",
			args:       []string{"go", "list", "-f", "{{.ImportPath}}", "./..."},
			wantOutput: "example.com/mod/pkg",
		},
		{
			name:       "PackageDirs",
			args:       []string{"go", "list", "-f", "{{.Dir}}", "./..."},
			wantOutput: "/root/mod/pkg",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := gatetest.GoList(
				"example.com/mod", "/root/mod",
				map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly("/root/mod/pkg")},
			)
			cmd := cmdrunner.NewCommand(".", tc.args[0], tc.args[1:]...)
			var stdout strings.Builder
			err := response(context.Background(), cmd, &stdout, io.Discard)
			if err != nil {
				t.Fatalf("GoList error: %v", err)
			}
			if strings.TrimSpace(stdout.String()) != tc.wantOutput {
				t.Fatalf("output = %q, want %q", strings.TrimSpace(stdout.String()), tc.wantOutput)
			}
		})
	}
}

func TestGoListScopedPatternFiltersByPackageDir(t *testing.T) {
	t.Parallel()
	response := gatetest.GoList(
		"example.com/mod",
		"/root/mod",
		map[string]gatetest.PackageListInfo{
			"example.com/mod/internal/app":     gatetest.DirOnly("/root/mod/internal/app"),
			"example.com/mod/pkg/internal/app": gatetest.DirOnly("/root/mod/pkg/internal/app"),
			"example.com/mod/vendor/lib":       gatetest.DirOnly("/root/mod/vendor/lib"),
		},
	)
	cmd := cmdrunner.NewCommand(".", "go", "list", "-f", "{{.ImportPath}}", "./internal/...")
	var stdout strings.Builder
	if err := response(context.Background(), cmd, &stdout, io.Discard); err != nil {
		t.Fatalf("GoList error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "example.com/mod/internal/app" {
		t.Fatalf("output = %q, want %q", got, "example.com/mod/internal/app")
	}
}

func TestGoListExactPatternFiltersByPackageDir(t *testing.T) {
	t.Parallel()
	response := gatetest.GoList(
		"example.com/mod",
		"/root/mod",
		map[string]gatetest.PackageListInfo{
			"example.com/mod/internal":     gatetest.DirOnly("/root/mod/internal"),
			"example.com/mod/internal/app": gatetest.DirOnly("/root/mod/internal/app"),
		},
	)
	cmd := cmdrunner.NewCommand(".", "go", "list", "-f", "{{.ImportPath}}", "./internal")
	var stdout strings.Builder
	if err := response(context.Background(), cmd, &stdout, io.Discard); err != nil {
		t.Fatalf("GoList error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "example.com/mod/internal" {
		t.Fatalf("output = %q, want %q", got, "example.com/mod/internal")
	}
}

func TestGoListPackageQueryRequiresExplicitPackageArg(t *testing.T) {
	t.Parallel()
	response := gatetest.GoList(
		"example.com/mod",
		"/root/mod",
		map[string]gatetest.PackageListInfo{"example.com/mod/pkg": gatetest.DirOnly("/root/mod/pkg")},
	)
	cmd := cmdrunner.NewCommand(".", "go", "list", "-f", "{{.ImportPath}}")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected missing package argument error")
	}
}

func TestGocyclo_OutputsScores(t *testing.T) {
	t.Parallel()
	response := gatetest.Gocyclo(map[string]int{"Validate": 5})
	cmd := cmdrunner.NewCommand(".", "go", "run", "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0", "-over", "0", "/pkg")
	var stdout strings.Builder
	err := response(context.Background(), cmd, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("Gocyclo error: %v", err)
	}
	want := "5 pkg Validate file.go:1:1\n"
	if strings.TrimSpace(stdout.String()) != strings.TrimSpace(want) {
		t.Fatalf("stdout = %q, want %q", strings.TrimSpace(stdout.String()), strings.TrimSpace(want))
	}
}

func TestGremlins_NoOutputFlag(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	response := gatetest.Gremlins(mem, "", []byte(`{}`))
	cmd := cmdrunner.NewCommand(".", "go", "run", "gremlins", "unleash")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("Gremlins error: %v", err)
	}
}

func TestGremlins_WritesRootRelativeCanonicalPath(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	report := []byte(`{"files":[]}`)
	root := "/module-root"
	const outLogical = "test-artifacts/mutations.json"
	response := gatetest.Gremlins(mem, root, report)
	cmd := cmdrunner.NewCommand(root, "gremlins", "unleash", "-o", outLogical)
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("Gremlins: %v", err)
	}
	want := outLogical
	data, readErr := mem.ReadFile(want)
	if readErr != nil {
		t.Fatalf("read mutation report from %q: %v", want, readErr)
	}
	if string(data) != `{"files":[]}` {
		t.Fatalf("report = %q", string(data))
	}
}

func TestGremlins_ParsesOutputPathEqualsAssign(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, ".")
	report := []byte(`{"ok":true}`)
	root := "/r"
	response := gatetest.Gremlins(mem, root, report)
	cmd := cmdrunner.NewCommand(root, "gremlins", "unleash", "-o=out/m.json")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("Gremlins: %v", err)
	}
	data, readErr := mem.ReadFile("out/m.json")
	if readErr != nil {
		t.Fatalf("read report: %v", readErr)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("content = %q", string(data))
	}
}

func TestGremlins_RejectsTraversalOutOfAnchor(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	response := gatetest.Gremlins(mem, "/gate", []byte(`{}`))
	cmd := cmdrunner.NewCommand(".", "gremlins", "unleash", "-o", "../outside.json")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if !errors.Is(err, gatetest.ErrGremlinsReportPathEscape) {
		t.Fatalf("errors.Is traversal: got %v", err)
	}
	if _, readErr := mem.ReadFile("../outside.json"); readErr == nil {
		t.Fatal("unexpected write outside anchor")
	}
}

func TestGremlins_RejectsAbsoluteOutputWhenAnchorIsDot(t *testing.T) {
	t.Parallel()
	for _, outPath := range []string{"/tmp/mutations.json", "C:/tmp/mutations.json"} {
		mem := gatetest.NewMemoryFileOps()
		response := gatetest.Gremlins(mem, ".", []byte(`{}`))
		cmd := cmdrunner.NewCommand(".", "gremlins", "unleash", "-o", outPath)
		err := response(context.Background(), cmd, io.Discard, io.Discard)
		if err == nil {
			t.Fatalf("expected absolute output %q to be rejected under dot anchor", outPath)
		}
		if !errors.Is(err, gatetest.ErrGremlinsReportPathEscape) {
			t.Fatalf("errors.Is ErrGremlinsReportPathEscape: got %v", err)
		}
	}
}
