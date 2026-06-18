package gatecheck

import (
	"errors"
	"reflect"
	"testing"
)

func testBuildGremlinsExcludeArgvHelper(
	t *testing.T,
	rows []MutationPackageRow,
	excludeSegments []string,
	testFilePatterns []string,
	want []string,
) {
	t.Helper()
	got, err := BuildGremlinsExcludeArgv(rows, nil, excludeSegments, testFilePatterns)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func commonTestRows() []MutationPackageRow {
	return []MutationPackageRow{
		{
			ImportPath: "example.com/mod/internal/app", PkgDirRootRel: "internal/app",
			GoFileNames: []string{"a.go"},
		},
		{
			ImportPath: "example.com/mod/internal/testutil", PkgDirRootRel: "internal/testutil",
			GoFileNames: []string{"util.go"},
		},
		{
			ImportPath: "example.com/mod/vendor/lib", PkgDirRootRel: "vendor/lib",
			GoFileNames: []string{"v.go"},
		},
	}
}

func TestBuildGremlinsExcludeArgv_BDD(t *testing.T) {
	t.Parallel()
	rows := commonTestRows()
	t.Run("testutil+vendor", func(t *testing.T) {
		t.Parallel()
		want := []string{
			`--exclude-files=^internal/testutil(/|$)`,
			`--exclude-files=^vendor/lib(/|$)`,
		}
		testBuildGremlinsExcludeArgvHelper(t, rows, []string{"testutil", "vendor"}, nil, want)
	})
	t.Run("overlapping", func(t *testing.T) {
		t.Parallel()
		want := []string{
			`--exclude-files=^internal/app(/|$)`,
			`--exclude-files=^internal/testutil(/|$)`,
		}
		testBuildGremlinsExcludeArgvHelper(t, rows, []string{"internal", "internal/testutil"}, nil, want)
	})
	t.Run("test_file", func(t *testing.T) {
		t.Parallel()
		r2 := []MutationPackageRow{{
			ImportPath:      "example.com/mod/internal/app",
			PkgDirRootRel:   "internal/app",
			GoFileNames:     []string{"a.go"},
			TestGoFileNames: []string{"foo_test.go", "bar_test.go"},
		}}
		want := []string{`--exclude-files=^internal/app/.*_test\.go$`}
		testBuildGremlinsExcludeArgvHelper(t, r2, nil, []string{"*_test.go"}, want)
	})
	t.Run("multiple_in_scope_packages_one_regex_each", func(t *testing.T) {
		t.Parallel()
		r2 := []MutationPackageRow{
			{
				ImportPath:      "example.com/mod/internal/app",
				PkgDirRootRel:   "internal/app",
				GoFileNames:     []string{"a.go"},
				TestGoFileNames: []string{"foo_test.go", "bar_test.go"},
			},
			{
				ImportPath:      "example.com/mod/vendor/lib",
				PkgDirRootRel:   "vendor/lib",
				GoFileNames:     []string{"v.go"},
				TestGoFileNames: []string{"v_test.go"},
			},
		}
		want := []string{
			`--exclude-files=^internal/app/.*_test\.go$`,
			`--exclude-files=^vendor/lib/.*_test\.go$`,
		}
		testBuildGremlinsExcludeArgvHelper(t, r2, nil, []string{"*_test.go"}, want)
	})
	t.Run("excluded_package_no_redundant_test_pattern", func(t *testing.T) {
		t.Parallel()
		r2 := []MutationPackageRow{
			{
				ImportPath:      "example.com/mod/internal/app",
				PkgDirRootRel:   "internal/app",
				GoFileNames:     []string{"a.go"},
				TestGoFileNames: []string{"foo_test.go"},
			},
			{
				ImportPath:      "example.com/mod/internal/testutil",
				PkgDirRootRel:   "internal/testutil",
				GoFileNames:     []string{"util.go"},
				TestGoFileNames: []string{"util_test.go"},
			},
		}
		want := []string{
			`--exclude-files=^internal/app/.*_test\.go$`,
			`--exclude-files=^internal/testutil(/|$)`,
		}
		testBuildGremlinsExcludeArgvHelper(t, r2, []string{"testutil"}, []string{"*_test.go"}, want)
	})
}

func TestBuildGremlinsExcludeArgv_rootModuleFiles(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod",
			PkgDirRootRel: ".",
			GoFileNames:   []string{"main.go", "keep.go", "root_extra+.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, []string{"main.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`--exclude-files=^main\.go$`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_rootPackageTestPattern(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:      "example.com/mod",
			PkgDirRootRel:   ".",
			GoFileNames:     []string{"main.go"},
			TestGoFileNames: []string{"main_test.go"},
		},
		{
			ImportPath:      "example.com/mod/internal/app",
			PkgDirRootRel:   "internal/app",
			GoFileNames:     []string{"a.go"},
			TestGoFileNames: []string{"a_test.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^[^/]*_test\.go$`,
		`--exclude-files=^internal/app/.*_test\.go$`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_metacharPath(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/p",
			PkgDirRootRel: "weird(a)/pkg",
			GoFileNames:   []string{"f.go", "g.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, []string{"weird(a)/pkg/f.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	// regexp.QuoteMeta escapes parentheses
	if got[0] != `--exclude-files=^weird\(a\)/pkg/f\.go$` {
		t.Fatalf("got %q", got[0])
	}
}

func TestBuildGremlinsExcludeArgv_slashNormalization(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/p",
			PkgDirRootRel: `a\b/c`,
			GoFileNames:   []string{"x.go", "y.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, []string{`a/b/c/x.go`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`--exclude-files=^a/b/c/x\.go$`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_excludesSourceFilesOutsideInventoryOnlyBySegment(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/pkg",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"pkg.go"},
		},
	}
	allSourceFiles := []string{
		"pkg/pkg.go",
		"magefiles/magefile.go",
		"testdata/failures/calc.go",
	}
	got, err := BuildGremlinsExcludeArgv(rows, allSourceFiles, []string{"testdata"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^testdata(/|$)`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_multipleUnlistedFilesCollapseToSegmentRegex(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/pkg",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"pkg.go"},
		},
	}
	allSourceFiles := []string{
		"pkg/pkg.go",
		"testdata/failures/calc.go",
		"testdata/failures/other.go",
		"testdata/deep/nested/x.go",
	}
	got, err := BuildGremlinsExcludeArgv(rows, allSourceFiles, []string{"testdata"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^testdata(/|$)`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_allNonTestExcludedByPattern(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/p",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"foo.go"},
		},
	}
	_, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"*.go"})
	if !errors.Is(err, ErrAllPackagesExcluded) {
		t.Fatalf("got err=%v want ErrAllPackagesExcluded", err)
	}
}

func TestBuildGremlinsExcludeArgv_allExcludedBySegments(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/a",
			PkgDirRootRel: "a",
			GoFileNames:   []string{"a.go"},
		},
		{
			ImportPath:    "example.com/mod/b",
			PkgDirRootRel: "b",
			GoFileNames:   []string{"b.go"},
		},
	}
	_, err := BuildGremlinsExcludeArgv(rows, nil, []string{"a", "b"}, nil)
	if !errors.Is(err, ErrAllPackagesExcluded) {
		t.Fatalf("got err=%v want ErrAllPackagesExcluded", err)
	}
}

func TestGremlinsExcludeFileArgs_nilOnAllExcluded(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{ImportPath: "p", PkgDirRootRel: "p", GoFileNames: []string{"x.go"}},
	}
	got, err := GremlinsExcludeFileArgs(rows, nil, []string{"*.go"})
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if got != nil {
		t.Fatalf("want nil argv, got %v", got)
	}
}

func TestGremlinsExcludeFileArgs_propagatesInvalidPattern(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{ImportPath: "p", PkgDirRootRel: "p", GoFileNames: []string{"x.go"}},
	}
	_, err := GremlinsExcludeFileArgs(rows, nil, []string{"[unclosed"})
	if !errors.Is(err, ErrInvalidTestFilePattern) {
		t.Fatalf("got err=%v want ErrInvalidTestFilePattern", err)
	}
}

func TestBuildGremlinsExcludeArgv_xTestFileNames(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:     "example.com/mod/pkg",
			PkgDirRootRel:  "pkg",
			GoFileNames:    []string{"p.go"},
			XTestFileNames: []string{"p_test.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`--exclude-files=^pkg/.*_test\.go$`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildGremlinsExcludeArgv_xTestFileNamesPatternMismatch(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:     "example.com/mod/pkg",
			PkgDirRootRel:  "pkg",
			GoFileNames:    []string{"p.go"},
			XTestFileNames: []string{"p_test.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"no_match_*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no excludes for non-matching pattern, got %v", got)
	}
}

func TestBuildGremlinsExcludeArgv_emptyTestFileNameSkipped(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:      "example.com/mod/pkg",
			PkgDirRootRel:   "pkg",
			GoFileNames:     []string{"p.go"},
			TestGoFileNames: []string{"", "  "},
			XTestFileNames:  []string{"", "  "},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no excludes for whitespace-only names, got %v", got)
	}
}

func TestBuildGremlinsExcludeArgv_invalidTestFilePattern(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/pkg",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"p.go"},
		},
	}
	_, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"[unclosed"})
	if err == nil {
		t.Fatal("expected error for malformed test file pattern")
	}
	if !errors.Is(err, ErrInvalidTestFilePattern) {
		t.Fatalf("BuildGremlinsExcludeArgv() err = %v, want ErrInvalidTestFilePattern", err)
	}
}

func TestBuildGremlinsExcludeArgv_emptyPatternSkipped(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:      "example.com/mod/pkg",
			PkgDirRootRel:   "pkg",
			GoFileNames:     []string{"p.go"},
			TestGoFileNames: []string{"p_test.go"},
		},
	}
	got, err := BuildGremlinsExcludeArgv(rows, nil, nil, []string{"", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no excludes for empty patterns, got %v", got)
	}
}
