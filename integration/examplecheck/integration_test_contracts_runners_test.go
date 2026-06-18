//go:build integration

package examplecheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	qg "github.com/hotchkj/mage-gate/gate"
)

type fixtureFailureStep struct {
	fixtureDir  string
	stepName    string
	goldenBase  string
	runInSilent func(context.Context) error
}

func fixtureFailuresRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func runFixtureFailureContract(tc fixtureFailureStep) func(*testing.T) {
	return func(t *testing.T) {
		runFixtureFailureContractCase(t, tc)
	}
}

func runFixtureFailureContractCase(
	t *testing.T,
	tc fixtureFailureStep,
) {
	t.Helper()
	requireFixtureDir(t, tc.fixtureDir)
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	stdout, stderr, runErr, captureErr := captureStdoutStderr(func() error {
		return withChdir(tc.fixtureDir, func() error { return tc.runInSilent(ctx) })
	})
	if captureErr != nil {
		t.Fatalf("capture stdout/stderr: %v", captureErr)
	}
	if runErr == nil {
		t.Fatalf("expected %s step to fail", tc.stepName)
	}
	skipIfToolingMissing(t, runErr)
	assertErrorFixHintTuple(t, runErr, tc.stepName)
	normalizedStdout := normalizeAgentGolden(tc.stepName, stdout)
	normalizedStderr := normalizeAgentGolden(tc.stepName, stderr)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeGolden(t, tc.goldenBase+".stdout", normalizedStdout)
		writeGolden(t, tc.goldenBase+".stderr", normalizedStderr)
	}
	wantStdout := readGolden(t, tc.goldenBase+".stdout")
	wantStderr := readGolden(t, tc.goldenBase+".stderr")
	if normalizedStdout != wantStdout {
		t.Fatalf("stdout golden mismatch\nwant:\n%q\ngot:\n%q", wantStdout, normalizedStdout)
	}
	if normalizedStderr != wantStderr {
		t.Fatalf("stderr golden mismatch\nwant:\n%q\ngot:\n%q", wantStderr, normalizedStderr)
	}
}

func TestFixtureFailures_AgentLintExactStdoutStderrGolden(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	dir := filepath.Join(root, "testdata", "failures", "lint-fail")
	requireFixtureDir(t, dir)
	lintCfg := filepath.Join(dir, ".golangci.yml")

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	stdout, stderr, runErr, captureErr := captureStdoutStderr(func() error {
		return withChdir(dir, func() error {
			runner, resolver, _, fileOps, pathRoot := newProductionWiring(t)
			pkgScope, scopeErr := qg.NewPackageScope("./...")
			if scopeErr != nil {
				return fmt.Errorf("package scope: %w", scopeErr)
			}
			return qg.Lint(
				ctx,
				runner,
				resolver,
				fileOps,
				pathRoot,
				pkgScope,
				mustLintToolchain(t, qg.LintConfig(lintCfg), qg.LintToolSpec(lintToolSpec)),
			)
		})
	})
	if captureErr != nil {
		t.Fatalf("capture stdout/stderr: %v", captureErr)
	}
	if runErr == nil {
		t.Fatal("expected lint to fail on fixture")
	}
	skipIfToolingMissing(t, runErr)
	_ = requireDiagnosticError(t, runErr, "lint")

	const wantStdout = "Lint...\n"
	const wantStderrFile = "lint.stderr"
	wantStderr := readGolden(t, wantStderrFile)
	if normalizeAgentGolden("lint", stdout) != wantStdout {
		t.Fatalf("stdout golden mismatch\nwant:\n%q\ngot:\n%q", wantStdout, normalizeAgentGolden("lint", stdout))
	}
	gotStderr := normalizeAgentGolden("lint", stderr)
	if gotStderr != wantStderr {
		t.Fatalf("stderr golden mismatch\nwant:\n%q\ngot:\n%q", wantStderr, gotStderr)
	}
}
