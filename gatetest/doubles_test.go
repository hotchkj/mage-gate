// Vision: External tests for gatetest doubles—fake resolver, mem file ops, cmdtest-wired command helpers.
package gatetest_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/spf13/afero"
)

var errAliasTest = errors.New("alias test error")

func TestGatetest_FakeToolResolver_GoRunFallback(t *testing.T) {
	t.Parallel()
	resolver := gatetest.NewFakeToolResolver()
	binary, args, err := resolver.ResolveToolCommand(
		context.Background(), "golangci-lint", "github.com/golangci/golangci-lint@v1.0.0", nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if binary != "go" {
		t.Fatalf("expected go, got %q", binary)
	}
	if len(args) == 0 {
		t.Fatalf("expected args to contain 'run', got empty slice")
	}
	if args[0] != "run" {
		t.Fatalf("args[0] = %q, want \"run\"", args[0])
	}
}

func TestGatetest_FakeToolResolver_LocalMatch(t *testing.T) {
	t.Parallel()
	resolver := gatetest.NewFakeToolResolver()
	const spec = "github.com/golangci/golangci-lint@v1.0.0"
	resolver.SetLocalMatch("golangci-lint", spec, true)
	binary, _, err := resolver.ResolveToolCommand(context.Background(), "golangci-lint", spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if binary != "golangci-lint" {
		t.Fatalf("expected golangci-lint, got %q", binary)
	}
}

func TestGatetest_FakeToolResolverError_ReturnsError(t *testing.T) {
	t.Parallel()
	resolver := gatetest.NewFakeToolResolverError("injected failure")
	_, _, err := resolver.ResolveToolCommand(context.Background(), "tool", "spec@v1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGatetest_MemoryFileOps_WriteReadRoundtrip(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root("/test-root"); err != nil {
		t.Fatalf("Root: %v", err)
	}
	const path = "tmp/test/file.txt"
	data := []byte("hello")
	if err := mem.MkdirAll("tmp/test", 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := mem.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestGatetest_MemoryFileOps_MkdirTempEmptyDirStaysInDotTree(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root("/test-root"); err != nil {
		t.Fatalf("Root: %v", err)
	}
	tempDir, err := mem.MkdirTemp("", "mage-gate-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	if tempDir == "" {
		t.Fatal("MkdirTemp returned empty path")
	}
	if filepath.IsAbs(tempDir) {
		t.Fatalf("MkdirTemp returned OS-absolute path %q", tempDir)
	}
	if filepath.ToSlash(tempDir) != tempDir {
		t.Fatalf("MkdirTemp returned non-canonical path %q", tempDir)
	}
	const artifact = "artifact.txt"
	fullPath := tempDir + "/" + artifact
	data := []byte("ok")
	if writeErr := mem.WriteFile(fullPath, data, 0o600); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}
	got, err := mem.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("data = %q, want %q", got, data)
	}
}

// TestGatetest_MemoryFileOps_AbsoluteMkdirAllCorruptsWalk proves the afero
// MemMapFs footgun: MkdirAll with an absolute path registers children of "/"
// whose FileData names include the leading "/". Readdirnames strips it via
// filepath.Split, so Walk constructs a path without "/", which MemMapFs can't
// find. Walk then aborts before visiting any later siblings.
//
// This is the root cause of the mutation_runner_scope.feature:108 CI failure:
// ensureMutationArtifactDir used ResolveWithinRoot (→ filepath.Abs) to build
// an absolute path, MkdirAll'd it in MemMapFs, and the resulting "/Users/…"
// child of root "/" broke Walk before it reached the fixture .go files.
func TestGatetest_MemoryFileOps_AbsoluteMkdirAllCorruptsWalk(t *testing.T) {
	t.Parallel()
	mem := afero.NewMemMapFs()
	if err := mem.MkdirAll("src", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(mem, "src/main.go", []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Baseline: Walk(".") finds src/main.go.
	baseline := walkGoFiles(t, mem, true)
	if !baseline["src/main.go"] {
		t.Fatalf("baseline Walk missed src/main.go, found: %v", baseline)
	}

	// Simulate the old ensureMutationArtifactDir: MkdirAll an absolute path.
	// "/abs/deep/dir" registers "/abs" as a child of "/" in MemMapFs with key
	// "/abs" but Readdirnames returns base name "abs". Walk builds
	// filepath.Join(".", "abs") = "abs", which doesn't match key "/abs".
	if err := mem.MkdirAll("/abs/deep/dir", 0o700); err != nil {
		t.Fatal(err)
	}

	// Walk(".") now aborts on the phantom "abs" child because "abs" sorts
	// before "src". Propagating the error (the default walkFn pattern) causes
	// the entire Walk to fail before reaching src/main.go.
	corrupted := walkGoFiles(t, mem, false)
	if corrupted["src/main.go"] {
		t.Fatal("expected Walk to abort before src/main.go due to absolute-path pollution, but file was found")
	}

	// Skipping the phantom entry (the resilient walkFn pattern) lets Walk
	// continue past the corruption and find the file.
	resilient := walkGoFiles(t, mem, true)
	if !resilient["src/main.go"] {
		t.Fatalf("resilient Walk missed src/main.go, found: %v", resilient)
	}
}

// walkGoFiles walks "." collecting .go file paths. When skipErrors is true,
// walk errors (e.g. from phantom MemMapFs entries) are skipped; when false,
// they abort the walk and the function returns whatever was collected so far.
func walkGoFiles(t *testing.T, mem afero.Fs, skipErrors bool) map[string]bool {
	t.Helper()
	found := make(map[string]bool)
	err := afero.Walk(mem, ".", func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			if skipErrors {
				return filepath.SkipDir
			}
			return walkErr
		}
		slashed := filepath.ToSlash(path)
		if info != nil && !info.IsDir() && strings.HasSuffix(slashed, ".go") {
			found[slashed] = true
		}
		return nil
	})
	if err != nil && !skipErrors {
		return found
	}
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return found
}

func TestGatetest_NoopCommand_Succeeds(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(cmdtest.On("go build", gatetest.NoopCommand))
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGatetest_Fail_ReturnsError(t *testing.T) {
	t.Parallel()
	fn := gatetest.Fail(errAliasTest)
	err := fn(context.Background(), cmdrunner.NewCommand(".", "go"), io.Discard, io.Discard)
	if !errors.Is(err, errAliasTest) {
		t.Fatalf("error = %v, want %v", err, errAliasTest)
	}
}
