// Vision: artifactPaths projections — logical argv/FileOps/host seams and lexical containment.
package harness

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

func gatedRoot(tb testing.TB) string {
	tb.Helper()
	root, err := filepath.Abs("gate-root")
	if err != nil {
		tb.Fatalf("Abs gate root: %v", err)
	}
	return root
}

func mustMemOpsRoot(tb testing.TB, root string) *gatetest.MemoryFileOps {
	tb.Helper()
	m := gatetest.NewMemoryFileOps()
	if err := m.Root(root); err != nil {
		tb.Fatalf("Root: %v", err)
	}
	return m
}

func mustNewArtifactPaths(tb testing.TB, root, artifactSubdir string) artifactPaths {
	tb.Helper()
	ap, err := newArtifactPaths(root, artifactSubdir)
	if err != nil {
		tb.Fatalf("newArtifactPaths: %v", err)
	}
	return ap
}

func artifactPathsFromMkdirTemp(tb testing.TB, root string) artifactPaths {
	tb.Helper()
	fs := mustMemOpsRoot(tb, root)
	tempLogical, err := fs.MkdirTemp("", "artifacts-")
	if err != nil {
		tb.Fatalf("MkdirTemp: %v", err)
	}
	return mustNewArtifactPaths(tb, root, tempLogical)
}

func TestArtifactPaths_DirMatchesFileOpsPathZeroParts(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "artifacts", "dir"))
	dir := ap.Dir()
	if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, `\`) {
		t.Fatalf("Dir must not trail separator: %q", dir)
	}
	gotPath, err := ap.FileOpsPath()
	if err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
	if gotPath != dir {
		t.Fatalf("FileOpsPath()=%q Dir()=%q", gotPath, dir)
	}
}

func TestArtifactPaths_CommandPathMatchesFileOpsPath(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "out", "sub"))
	fo, err := ap.FileOpsPath("a", "b")
	if err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
	cp, err := ap.CommandPath("a", "b")
	if err != nil {
		t.Fatalf("CommandPath: %v", err)
	}
	if cp != fo {
		t.Fatalf("CommandPath=%q FileOpsPath=%q", cp, fo)
	}
}

func TestArtifactPaths_FileOpsPath_MixedSeparators(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "artifacts", "base"))
	got, err := ap.FileOpsPath(`sub\mixed/x.out`)
	if err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
	if want := "artifacts/base/sub/mixed/x.out"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestArtifactPaths_ValidateTraversal(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "artifacts", "proj"))
	if err := ap.Validate("..", "outside"); err == nil {
		t.Fatal("expected Validate traversal error")
	} else if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_FileOpsPath_SubTraversal(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "artifacts", "proj"))
	_, err := ap.FileOpsPath("sub", "..", "..", "..", "etc")
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_AbsoluteArtifactSubdirUnderRoot(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, `abs-artifacts`))
	if _, err := ap.FileOpsPath("x"); err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
	if got := fsnorm.Base(ap.Dir()); got != "abs-artifacts" {
		t.Fatalf("Dir() basename=%q, full %q", got, ap.Dir())
	}
}

func TestArtifactPaths_AbsoluteOutsideRootRejected(t *testing.T) {
	t.Parallel()
	dirA, err := filepath.Abs("proj-a")
	if err != nil {
		t.Fatalf("Abs proj-a: %v", err)
	}
	dirB, err := filepath.Abs("proj-b")
	if err != nil {
		t.Fatalf("Abs proj-b: %v", err)
	}
	if _, err := newArtifactPaths(dirA, filepath.Join(dirB, "evil")); err == nil {
		t.Fatal("expected rejection for subtree outside configured root")
	}
}

func TestArtifactPaths_HostPathMatchesResolveWithinRoot(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := mustNewArtifactPaths(t, root, filepath.Join(root, "artifacts", "tmp"))
	lp, ferr := ap.FileOpsPath(`nest\doc.txt`)
	if ferr != nil {
		t.Fatalf("FileOpsPath: %v", ferr)
	}
	host, herr := ap.HostPath(`nest\doc.txt`)
	if herr != nil {
		t.Fatalf("HostPath: %v", herr)
	}
	wantHost, wantErr := ResolveWithinRoot(root, logicalSegmentsForResolve(lp)...)
	if wantErr != nil {
		t.Fatalf("ResolveWithinRoot(want): %v", wantErr)
	}
	if host != wantHost {
		t.Fatalf("HostPath=%q ResolveWithinRoot=%q", host, wantHost)
	}
}

func TestArtifactPaths_FromMkdirTempUnderRoot(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	ap := artifactPathsFromMkdirTemp(t, root)
	dir := ap.Dir()
	if strings.HasSuffix(dir, "/") {
		t.Fatalf("Dir must not trail slash: %q", dir)
	}
	got, err := ap.FileOpsPath("file.txt")
	if err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
	if want := fsnorm.Join(dir, "file.txt"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if err := ap.Validate(); err != nil {
		t.Fatalf("Validate empty: %v", err)
	}
}

func TestNewStepRunner_WiresArtifactPathsLogicalDir(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	sub := filepath.Join(root, `artifact-sub`)
	stepRunner, err := NewStepRunner(
		root,
		sub,
		`./...`,
		cmdtest.NewFakeRunner(),
		mustMemOpsRoot(t, root),
		NewDiscardArtifactStore(),
		"",
	)
	if err != nil {
		t.Fatalf("NewStepRunner: %v", err)
	}
	want, ferr := fileopspath.LogicalContainedRelative(root, sub)
	if ferr != nil {
		t.Fatalf("LogicalContainedRelative: %v", ferr)
	}
	if got := ArtifactLogicalDirForTest(stepRunner); got != want {
		t.Fatalf("logical dir=%q want %q", got, want)
	}
}

func TestNewStepRunner_RejectsTraversalArtifactSubdir(t *testing.T) {
	t.Parallel()
	root := gatedRoot(t)
	_, err := NewStepRunner(
		root,
		filepath.Join(root, "..", "outside-artifacts"),
		`./...`,
		cmdtest.NewFakeRunner(),
		mustMemOpsRoot(t, root),
		NewDiscardArtifactStore(),
		"",
	)
	if err == nil {
		t.Fatal("expected NewStepRunner error")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}
