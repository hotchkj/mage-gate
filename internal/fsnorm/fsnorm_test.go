package fsnorm

import (
	"errors"
	"strings"
	"testing"
)

func TestCanonical_Empty(t *testing.T) {
	t.Parallel()
	if got := Canonical(""); got != "" {
		t.Fatalf("Canonical(\"\") = %q, want \"\"", got)
	}
}

func TestCanonical_RelativeBackslashes(t *testing.T) {
	t.Parallel()
	const want = "internal/harness/config.go"
	if got := Canonical(`internal\harness\config.go`); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_MixedSeparators(t *testing.T) {
	t.Parallel()
	const want = "internal/harness/config.go"
	if got := Canonical(`internal\harness/config.go`); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_UnixAbsolute(t *testing.T) {
	t.Parallel()
	const want = "/home/dev/repo/internal/harness/config.go"
	if got := Canonical("/home/dev/repo/internal/harness/config.go"); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_WindowsStyleDrivePath(t *testing.T) {
	t.Parallel()
	const in = `C:\Users\dev\repo\internal\harness\config.go`
	const want = "C:/Users/dev/repo/internal/harness/config.go"
	if got := Canonical(in); got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DotSegments(t *testing.T) {
	t.Parallel()
	if got, want := Canonical("x/y/../z"), "x/z"; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_Idempotent(t *testing.T) {
	t.Parallel()
	const in = `C:\a\b\..\c`
	once := Canonical(in)
	twice := Canonical(once)
	if once != twice {
		t.Fatalf("Canonical not idempotent: once=%q twice=%q", once, twice)
	}
}

func TestCanonical_TrailingSlash(t *testing.T) {
	t.Parallel()
	if got, want := Canonical("a/b/"), "a/b"; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DotOnly(t *testing.T) {
	t.Parallel()
	if got, want := Canonical("."), "."; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_DoubleDotOnly(t *testing.T) {
	t.Parallel()
	if got, want := Canonical(".."), ".."; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
}

func TestCanonical_UNCStyleDoubleSlash(t *testing.T) {
	t.Parallel()
	got := Canonical(`//host/share/path`)
	if got == "" {
		t.Fatal("unexpected empty")
	}
	if Canonical(got) != got {
		t.Fatalf("expected idempotent, got follow-up %q", Canonical(got))
	}
	// filepath.Clean collapses duplicate slashes in the prefix (GOOS-dependent details).
	if !strings.HasPrefix(got, "/") {
		t.Fatalf("expected rooted slash form, got %q", got)
	}
}

func TestRel_DotRootRelativePath(t *testing.T) {
	t.Parallel()
	got, err := Rel(".", "testdata/failures/calc.go")
	if err != nil {
		t.Fatal(err)
	}
	const want = "testdata/failures/calc.go"
	if got != want {
		t.Fatalf("Rel = %q, want %q", got, want)
	}
}

func TestRel_UnixStyleAbsoluteFakes(t *testing.T) {
	t.Parallel()
	got, err := Rel("/mod", "/mod/internal/app")
	if err != nil {
		t.Fatal(err)
	}
	const want = "internal/app"
	if got != want {
		t.Fatalf("Rel = %q, want %q", got, want)
	}
}

func TestRel_WindowsDriveLexicalPath(t *testing.T) {
	t.Parallel()
	got, err := Rel("C:/repo", "C:/repo/internal/app")
	if err != nil {
		t.Fatal(err)
	}
	const want = "internal/app"
	if got != want {
		t.Fatalf("Rel = %q, want %q", got, want)
	}
}

const testFileName = "file.go"

func TestRel_EmptyBaseDefaultsToDot(t *testing.T) {
	t.Parallel()
	got, err := Rel("", "internal/app")
	if err != nil {
		t.Fatal(err)
	}
	const want = "internal/app"
	if got != want {
		t.Fatalf("Rel = %q, want %q", got, want)
	}
}

func TestRel_PathMismatchReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Rel("/repo", "repo/internal/app")
	if !errors.Is(err, ErrRelPathMismatch) {
		t.Fatalf("expected ErrRelPathMismatch, got %v", err)
	}
}

func TestRel_CanonicalCornerCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		base   string
		target string
		want   string
	}{
		{
			name:   "same relative path",
			base:   "internal/app",
			target: "internal/app",
			want:   ".",
		},
		{
			name:   "relative child",
			base:   "internal/app",
			target: "internal/app/foo.go",
			want:   "foo.go",
		},
		{
			name:   "relative parent",
			base:   "internal/app/sub",
			target: "internal/app",
			want:   "..",
		},
		{
			name:   "relative sibling",
			base:   "internal/app",
			target: "internal/testutil",
			want:   "../testutil",
		},
		{
			name:   "absolute sibling",
			base:   "/repo/internal/app",
			target: "/repo/internal/testutil/pkg.go",
			want:   "../testutil/pkg.go",
		},
		{
			name:   "canonicalizes dot segments",
			base:   "internal/app/../app",
			target: "internal/app/./foo.go",
			want:   "foo.go",
		},
		{
			name:   "canonicalizes backslashes",
			base:   `internal\app`,
			target: `internal\testutil\pkg.go`,
			want:   "../testutil/pkg.go",
		},
		{
			name:   "case-insensitive drive volume",
			base:   "C:/repo/internal/app",
			target: "c:/repo/internal/testutil/pkg.go",
			want:   "../testutil/pkg.go",
		},
		{
			name:   "double dot base",
			base:   "../fixture",
			target: "../fixture/pkg/file.go",
			want:   "pkg/file.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Rel(tt.base, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Rel(%q, %q) = %q, want %q", tt.base, tt.target, got, tt.want)
			}
		})
	}
}

func TestRel_MismatchCornerCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		base   string
		target string
	}{
		{
			name:   "rooted base with relative target",
			base:   "/repo",
			target: "repo/internal/app",
		},
		{
			name:   "relative base with rooted target",
			base:   "repo",
			target: "/repo/internal/app",
		},
		{
			name:   "different drive volumes",
			base:   "C:/repo",
			target: "D:/repo/internal/app",
		},
		{
			name:   "drive volume versus rooted path",
			base:   "C:/repo",
			target: "/repo/internal/app",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Rel(tt.base, tt.target)
			if !errors.Is(err, ErrRelPathMismatch) {
				t.Fatalf("expected ErrRelPathMismatch, got %v", err)
			}
		})
	}
}

func TestDir_CanonicalPath(t *testing.T) {
	t.Parallel()
	if got, want := Dir(`testdata\failures\calc.go`), "testdata/failures"; got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestJoin_CanonicalPath(t *testing.T) {
	t.Parallel()
	if got, want := Join(`testdata\failures`, "calc.go"), "testdata/failures/calc.go"; got != want {
		t.Fatalf("Join = %q, want %q", got, want)
	}
}

func TestBase_CanonicalPath(t *testing.T) {
	t.Parallel()
	if got, want := Base(`testdata\failures\calc.go`), "calc.go"; got != want {
		t.Fatalf("Base = %q, want %q", got, want)
	}
}

// Tests for previously surviving mutations in internal/fsnorm/fsnorm.go

func TestRel_SameRootedStatus(t *testing.T) {
	t.Parallel()
	// Both rooted - line 42 condition true path (isRooted equal)
	got, err := Rel("/a/b", "/a/b/c")
	if err != nil {
		t.Fatalf("Rel rooted pair: %v", err)
	}
	if got != "c" {
		t.Fatalf("Rel = %q, want %q", got, "c")
	}
}

func TestRel_MismatchedRootedStatus(t *testing.T) {
	t.Parallel()
	// Line 42: isRooted(baseRest) != isRooted(targetRest) - false path triggers mismatch
	tests := []struct {
		name   string
		base   string
		target string
	}{
		{
			name:   "rooted base unrooted target",
			base:   "/repo",
			target: "repo/internal",
		},
		{
			name:   "unrooted base rooted target",
			base:   "repo",
			target: "/repo/internal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Rel(tt.base, tt.target)
			if !errors.Is(err, ErrRelPathMismatch) {
				t.Fatalf("expected ErrRelPathMismatch, got %v", err)
			}
		})
	}
}

func TestRel_DifferentDriveVolumes(t *testing.T) {
	t.Parallel()
	// Line 42: baseVol != targetVol triggers mismatch
	_, err := Rel("C:/repo", "D:/repo/file.go")
	if !errors.Is(err, ErrRelPathMismatch) {
		t.Fatalf("expected ErrRelPathMismatch for different drives, got %v", err)
	}
}

func TestCanonicalRelDefault_EmptyPath(t *testing.T) {
	t.Parallel()
	// Line 86: empty canonical path defaults to "."
	got, err := Rel("", testFileName)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != testFileName {
		t.Fatalf("Rel(\"\", %q) = %q, want %q", testFileName, got, testFileName)
	}
}

func TestSplitCanonicalVolume_LowercaseDrive(t *testing.T) {
	t.Parallel()
	// Lines 93-96: lowercase drive letter handling (isASCIIAlpha + ToUpper)
	got, err := Rel("c:/repo", "c:/repo/"+testFileName)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != testFileName {
		t.Fatalf("Rel = %q, want %q", got, testFileName)
	}
}

func TestSplitCanonicalVolume_NonAlphaDrivePrefix(t *testing.T) {
	t.Parallel()
	// Lines 93-96, 99-101: isASCIIAlpha ensures non-alpha doesn't parse as volume
	// "1:" is not a valid volume, so treated as relative path
	got, err := Rel("1:/repo", "1:/repo/"+testFileName)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != testFileName {
		t.Fatalf("Rel = %q, want %q", got, testFileName)
	}
}

func TestSplitCanonicalParts_EmptyAfterSlash(t *testing.T) {
	t.Parallel()
	// Lines 107-112: splitCanonicalParts with empty after trim
	got, err := Rel("/", "/"+testFileName)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != testFileName {
		t.Fatalf("Rel = %q, want %q", got, testFileName)
	}
}

func TestSplitCanonicalParts_DotOnly(t *testing.T) {
	t.Parallel()
	// Line 109: path equals "." returns nil
	got, err := Rel(".", "./"+testFileName)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != testFileName {
		t.Fatalf("Rel = %q, want %q", got, testFileName)
	}
}

func TestCommonPrefixLen_EqualLengthArrays(t *testing.T) {
	t.Parallel()
	// Lines 116-125: commonPrefixLen with equal length arrays
	// Rel("a/b/c", "a/b/d") should produce "../d" not "../b/d"
	got, err := Rel("a/b/c", "a/b/d")
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != "../d" {
		t.Fatalf("Rel = %q, want %q", got, "../d")
	}
}

func TestCommonPrefixLen_LeftShorter(t *testing.T) {
	t.Parallel()
	// Lines 116-125: left shorter than right
	got, err := Rel("a/b", "a/b/c/d")
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if got != "c/d" {
		t.Fatalf("Rel = %q, want %q", got, "c/d")
	}
}

func TestJoin_NoParts(t *testing.T) {
	t.Parallel()
	// Line 68: Join with no parts returns empty
	if got := Join(); got != "" {
		t.Fatalf("Join() = %q, want empty", got)
	}
}
