package fileopspath_test

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
)

func TestLogicalContainedRelative_MixedSeparators(t *testing.T) {
	t.Parallel()
	const root = "/test-root"
	got, err := fileopspath.LogicalContainedRelative(root, `artifacts\coverage.out`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got != "artifacts/coverage.out" {
		t.Fatalf("got %q", got)
	}
}

func TestLogicalContainedRelative_Traversal(t *testing.T) {
	t.Parallel()
	const root = "/test-root"
	_, err := fileopspath.LogicalContainedRelative(root, "../outside")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fileopspath.ErrPathTraversal) {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLogicalContainedRelative_EmptyResolvesToDot(t *testing.T) {
	t.Parallel()
	got, err := fileopspath.LogicalContainedRelative("/solo", "")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got != "." {
		t.Fatalf("got %q want .", got)
	}
}

func TestLogicalContainedRelative_UnixRootRepeated(t *testing.T) {
	t.Parallel()
	got, err := fileopspath.LogicalContainedRelative("/test-root", "/test-root")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got != "." {
		t.Fatalf("got %q want .", got)
	}
}

func TestLogicalContainedRelative_WindowsDifferentHostDrive(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("host drive letters are Windows-only")
	}
	_, err := fileopspath.LogicalContainedRelative(`C:\proj`, `D:\other\file.go`)
	if err == nil {
		t.Fatal("expected error for different volume")
	}
}

func TestLogicalContainedRelative_LongLogicalPath(t *testing.T) {
	t.Parallel()
	root := "/test-root"
	seg := strings.Repeat("deep/", 240) + "end.txt"
	got, err := fileopspath.LogicalContainedRelative(root, seg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if filepath.Base(got) != "end.txt" {
		t.Fatalf("tail got %q", got)
	}
}
