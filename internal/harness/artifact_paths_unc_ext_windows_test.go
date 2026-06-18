// Vision: Windows UNC and extended-length (\\?\) artifact path containment without real shares or disk IO.
//
//go:build windows

package harness

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
)

func testExtendedLengthDriveRootsAvailable(tb testing.TB) (cLex, dLex string, skip bool, skipReason string) {
	tb.Helper()
	// Deliberately synthetic — Go’s filepath.Abs normalizes these lexically; directories need not exist.
	cLex = `\\?\C:\mg-gate-lex\c-root-not-present`
	dLex = `\\?\D:\mg-gate-lex\d-root-not-present`
	if _, err := filepath.Abs(filepath.Clean(cLex)); err != nil {
		return "", "", true, "blocker: filepath.Abs(\\?\\C:\\…): " + err.Error()
	}
	if _, err := filepath.Abs(filepath.Clean(dLex)); err != nil {
		return "", "", true, "blocker: filepath.Abs(\\?\\D:\\…): " + err.Error()
	}
	return cLex, dLex, false, ""
}

func TestArtifactPaths_WindowsDifferentDriveRejected(t *testing.T) {
	t.Parallel()
	_, err := newArtifactPaths(`C:\proj`, `D:\other\tmp`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_UNC_sameServerSameShare_underRootAccepted(t *testing.T) {
	t.Parallel()
	const rootLex = `\\mg-gate.gateway.internal\harness-share\gates\demo`
	artifactLex := filepath.Join(rootLex, `build`, `_artifacts`)

	wantDir, werr := fileopspath.LogicalContainedRelative(rootLex, artifactLex)
	if werr != nil {
		t.Fatalf("LogicalContainedRelative (%q, %q): %v", rootLex, artifactLex, werr)
	}

	ap, err := newArtifactPaths(rootLex, artifactLex)
	if err != nil {
		t.Fatalf("newArtifactPaths: %v", err)
	}
	if ap.Dir() != wantDir {
		t.Fatalf("Dir()=%q want %q", ap.Dir(), wantDir)
	}

	if _, fileOpsErr := ap.FileOpsPath(`nest`, `unit.out`); fileOpsErr != nil {
		t.Fatalf("FileOpsPath: %v", fileOpsErr)
	}

	lp, err := ap.FileOpsPath(`nest`, `unit.out`)
	if err != nil {
		t.Fatalf("FileOpsPath repeat: %v", err)
	}
	host, err := ap.HostPath(`nest`, `unit.out`)
	if err != nil {
		t.Fatalf("HostPath: %v", err)
	}
	wantHost, herr := ResolveWithinRoot(rootLex, logicalSegmentsForResolve(lp)...)
	if herr != nil {
		t.Fatalf("ResolveWithinRoot: %v", herr)
	}
	if host != wantHost {
		t.Fatalf("HostPath=%q ResolveWithinRoot=%q", host, wantHost)
	}
}

func TestArtifactPaths_UNC_differentShareRejected(t *testing.T) {
	t.Parallel()
	const rootLex = `\\samehost.mg\share_alpha\repo`
	artifactLex := `\\samehost.mg\share_beta\repo\_out`
	_, err := newArtifactPaths(rootLex, artifactLex)
	if err == nil {
		t.Fatal("expected error for UNC path on a different share name")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_UNC_differentServerRejected(t *testing.T) {
	t.Parallel()
	const rootLex = `\\fs-east.mg\vol\workspace`
	artifactLex := `\\fs-west.mg\vol\workspace\artifacts`
	_, err := newArtifactPaths(rootLex, artifactLex)
	if err == nil {
		t.Fatal("expected error for different UNC server component")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_ExtendedLength_sameVolumeUnderRootAccepted(t *testing.T) {
	t.Parallel()
	cRoot, _, skip, why := testExtendedLengthDriveRootsAvailable(t)
	if skip {
		t.Skip(why)
	}
	artifactLex := filepath.Join(cRoot, `out`, `staging`)

	wantDir, ferr := fileopspath.LogicalContainedRelative(cRoot, filepath.Clean(artifactLex))
	if ferr != nil {
		t.Fatalf("LogicalContainedRelative: %v", ferr)
	}
	ap, err := newArtifactPaths(cRoot, filepath.Clean(artifactLex))
	if err != nil {
		t.Fatalf("newArtifactPaths: %v", err)
	}
	if ap.Dir() != wantDir {
		t.Fatalf("Dir()=%q want %q", ap.Dir(), wantDir)
	}
	if _, err := ap.FileOpsPath(`x.go`); err != nil {
		t.Fatalf("FileOpsPath: %v", err)
	}
}

func TestArtifactPaths_ExtendedLength_differentDriveRejected(t *testing.T) {
	t.Parallel()
	cRoot, dRoot, skip, why := testExtendedLengthDriveRootsAvailable(t)
	if skip {
		t.Skip(why)
	}
	artifactLex := filepath.Join(dRoot, `under-d`, `artifacts`)
	_, err := newArtifactPaths(cRoot, filepath.Clean(artifactLex))
	if err == nil {
		t.Fatal("expected error when artifact resolves on a different extended-length drive than root")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestArtifactPaths_ExtendedLength_peerDirectorySameVolumeRejected(t *testing.T) {
	t.Parallel()
	cRoot, _, skip, why := testExtendedLengthDriveRootsAvailable(t)
	if skip {
		t.Skip(why)
	}
	parent := filepath.Dir(cRoot)
	if parent == "." || len(parent) < 4 {
		t.Fatalf("unexpected Dir(extended-root): raw=%q Dir=%q", cRoot, parent)
	}
	artifactLex := filepath.Join(parent, `foreign-peer-not-under-root`, `bin`)
	_, err := newArtifactPaths(cRoot, filepath.Clean(artifactLex))
	if err == nil {
		t.Fatal("expected error for sibling subtree outside extended-length root")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}
