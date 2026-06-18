// Vision: Quality-scope command command-scope projections stay aligned across gate steps.
package gatecheck

import (
	"errors"
	"reflect"
	"testing"
)

func bddQualityScopeRows() []MutationPackageRow {
	return []MutationPackageRow{
		{
			ImportPath:      "example.com/mod/zz_bdd_not_default/pkg",
			PkgDirRootRel:   "zz_bdd_not_default/pkg",
			GoFileNames:     []string{"pkg.go"},
			TestGoFileNames: []string{"pkg_test.go"},
		},
		{
			ImportPath:      "example.com/mod/zz_bdd_not_default/testutil",
			PkgDirRootRel:   "zz_bdd_not_default/testutil",
			GoFileNames:     []string{"util.go"},
			TestGoFileNames: []string{"util_test.go"},
		},
	}
}

func TestNewQualityScopeCommandScope_ParseExcludeSegmentsOnce(t *testing.T) {
	t.Parallel()
	commandScope := NewQualityScopeCommandScope(
		bddQualityScopeRows(), nil, ` vendor ,internal/testutil `, nil, nil,
	)
	got := commandScope.ExcludeSegments()
	want := []string{"vendor", "internal/testutil"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExcludeSegments() = %v, want %v", got, want)
	}
}

func TestQualityScopeCommandScope_Tags(t *testing.T) {
	t.Parallel()
	commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", nil, []string{"mage", "integration"})
	if got := commandScope.TagsCSV(); got != "mage,integration" {
		t.Fatalf("TagsCSV() = %q, want mage,integration", got)
	}
	if !reflect.DeepEqual(commandScope.Tags(), []string{"mage", "integration"}) {
		t.Fatalf("Tags() = %v", commandScope.Tags())
	}
}

func TestQualityScopeCommandScope_CoverpkgCSV(t *testing.T) {
	t.Parallel()
	t.Run("no_excludes", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", nil, nil)
		got, err := commandScope.CoverpkgCSV()
		if err != nil {
			t.Fatal(err)
		}
		want := "example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil"
		if got != want {
			t.Fatalf("CoverpkgCSV() = %q, want %q", got, want)
		}
	})
	t.Run("excludes_testutil", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "testutil", nil, nil)
		got, err := commandScope.CoverpkgCSV()
		if err != nil {
			t.Fatal(err)
		}
		want := "example.com/mod/zz_bdd_not_default/pkg"
		if got != want {
			t.Fatalf("CoverpkgCSV() = %q, want %q", got, want)
		}
	})
	t.Run("all_excluded", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "zz_bdd_not_default", nil, nil)
		_, err := commandScope.CoverpkgCSV()
		if !errors.Is(err, ErrAllPackagesExcluded) {
			t.Fatalf("CoverpkgCSV() err = %v, want ErrAllPackagesExcluded", err)
		}
	})
}

func TestQualityScopeCommandScope_CoverageProfileFilter(t *testing.T) {
	t.Parallel()
	t.Run("not_needed", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", nil, nil)
		filter := commandScope.CoverageProfileFilter()
		if filter.Needed() {
			t.Fatal("expected no profile filtering")
		}
		input := "mode: set\npkg/a.go:1.1,2.2 3 1\n"
		out, err := filter.Apply(input)
		if err != nil {
			t.Fatal(err)
		}
		if out != input {
			t.Fatalf("Apply() = %q, want unchanged", out)
		}
	})
	t.Run("exclude_segment", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "testutil", nil, nil)
		filter := commandScope.CoverageProfileFilter()
		if !filter.Needed() {
			t.Fatal("expected profile filtering")
		}
		input := "mode: set\n" +
			"zz_bdd_not_default/pkg/pkg.go:1.1,2.2 3 1\n" +
			"zz_bdd_not_default/testutil/util.go:1.1,2.2 3 1\n"
		out, err := filter.Apply(input)
		if err != nil {
			t.Fatal(err)
		}
		want := "mode: set\nzz_bdd_not_default/pkg/pkg.go:1.1,2.2 3 1\n"
		if out != want {
			t.Fatalf("Apply() = %q, want %q", out, want)
		}
	})
	t.Run("test_file_pattern", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", []string{"*_test.go"}, nil)
		filter := commandScope.CoverageProfileFilter()
		if !filter.Needed() {
			t.Fatal("expected profile filtering for test patterns")
		}
	})
}

func TestQualityScopeCommandScope_ThresholdPathFilters(t *testing.T) {
	t.Parallel()
	commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "vendor", []string{"*_test.go"}, nil)
	exSegs, patterns := commandScope.ThresholdPathFilters()
	if !reflect.DeepEqual(exSegs, []string{"vendor"}) {
		t.Fatalf("exclude segments = %v", exSegs)
	}
	if !reflect.DeepEqual(patterns, []string{"*_test.go"}) {
		t.Fatalf("test patterns = %v", patterns)
	}
}

func TestQualityScopeCommandScope_GocycloPkgDirsRootRel(t *testing.T) {
	t.Parallel()
	t.Run("all_in_scope", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "", nil, nil)
		got, err := commandScope.GocycloPkgDirsRootRel()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"zz_bdd_not_default/pkg", "zz_bdd_not_default/testutil"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("GocycloPkgDirsRootRel() = %v, want %v", got, want)
		}
	})
	t.Run("excludes_narrow_dirs", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "testutil", nil, nil)
		got, err := commandScope.GocycloPkgDirsRootRel()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"zz_bdd_not_default/pkg"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("GocycloPkgDirsRootRel() = %v, want %v", got, want)
		}
	})
	t.Run("all_excluded", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(bddQualityScopeRows(), nil, "zz_bdd_not_default", nil, nil)
		_, err := commandScope.GocycloPkgDirsRootRel()
		if !errors.Is(err, ErrAllPackagesExcluded) {
			t.Fatalf("GocycloPkgDirsRootRel() err = %v, want ErrAllPackagesExcluded", err)
		}
	})
}

func TestQualityScopeCommandScope_MutationExcludeFileArgv_Inventory(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/pkg",
			PkgDirRootRel: "pkg",
			GoFileNames:   []string{"pkg.go"},
		},
	}
	sourceFiles := []string{"pkg/pkg.go", "magefiles/magefile.go", "testdata/failures/calc.go"}

	t.Run("requires_source_inventory", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(rows, nil, "testdata", nil, nil)
		_, err := commandScope.MutationExcludeFileArgv()
		if !errors.Is(err, ErrQualityScopeSourceInventoryRequired) {
			t.Fatalf("MutationExcludeFileArgv() err = %v, want ErrQualityScopeSourceInventoryRequired", err)
		}
	})
	t.Run("empty_source_inventory_supplied", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(rows, []string{}, "testdata", nil, nil)
		_, err := commandScope.MutationExcludeFileArgv()
		if err != nil {
			t.Fatalf("empty but supplied source inventory must succeed, got %v", err)
		}
	})
	t.Run("excludes_source_outside_packages", func(t *testing.T) {
		t.Parallel()
		commandScope := NewQualityScopeCommandScope(rows, sourceFiles, "testdata", nil, nil)
		got, err := commandScope.MutationExcludeFileArgv()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{`--exclude-files=^testdata(/|$)`}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("package_exclude_without_source_inventory", func(t *testing.T) {
		t.Parallel()
		rows := []MutationPackageRow{
			{
				ImportPath:    "example.com/mod/internal/app",
				PkgDirRootRel: "internal/app",
				GoFileNames:   []string{"a.go"},
			},
			{
				ImportPath:    "example.com/mod/internal/testutil",
				PkgDirRootRel: "internal/testutil",
				GoFileNames:   []string{"util.go"},
			},
		}
		commandScope := NewQualityScopeCommandScope(rows, nil, "testutil", nil, nil)
		got, err := commandScope.MutationExcludeFileArgv()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{`--exclude-files=^internal/testutil(/|$)`}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

func TestQualityScopeCommandScope_MutationExcludeFileArgv_PackageDir(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/internal/app",
			PkgDirRootRel: "internal/app",
			GoFileNames:   []string{"a.go"},
		},
		{
			ImportPath:    "example.com/mod/internal/testutil",
			PkgDirRootRel: "internal/testutil",
			GoFileNames:   []string{"util.go"},
		},
	}
	commandScope := NewQualityScopeCommandScope(
		rows,
		[]string{"internal/app/a.go", "internal/testutil/util.go"},
		"testutil",
		nil,
		nil,
	)
	got, err := commandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`--exclude-files=^internal/testutil(/|$)`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestQualityScopeCommandScope_MutationExcludeFileArgv_Overlapping(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{
		{
			ImportPath:    "example.com/mod/internal/app",
			PkgDirRootRel: "internal/app",
			GoFileNames:   []string{"a.go"},
		},
		{
			ImportPath:    "example.com/mod/internal/testutil",
			PkgDirRootRel: "internal/testutil",
			GoFileNames:   []string{"util.go"},
		},
		{
			ImportPath:    "example.com/mod/vendor/lib",
			PkgDirRootRel: "vendor/lib",
			GoFileNames:   []string{"v.go"},
		},
	}
	commandScope := NewQualityScopeCommandScope(rows, nil, "internal,internal/testutil", nil, nil)
	got, err := commandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^internal/app(/|$)`,
		`--exclude-files=^internal/testutil(/|$)`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestQualityScopeCommandScope_MutationExcludeFileArgv_TestPattern(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{{
		ImportPath:      "example.com/mod/internal/app",
		PkgDirRootRel:   "internal/app",
		GoFileNames:     []string{"a.go"},
		TestGoFileNames: []string{"foo_test.go"},
	}}
	commandScope := NewQualityScopeCommandScope(rows, []string{"internal/app/a.go"}, "", []string{"*_test.go"}, nil)
	got, err := commandScope.MutationExcludeFileArgv()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`--exclude-files=^internal/app/.*_test\.go$`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestQualityScopeCommandScope_MutationExcludeFileArgv_AllExcluded(t *testing.T) {
	t.Parallel()
	rows := []MutationPackageRow{{
		ImportPath:    "example.com/mod/p",
		PkgDirRootRel: "p",
		GoFileNames:   []string{"foo.go"},
	}}
	commandScope := NewQualityScopeCommandScope(rows, []string{"p/foo.go"}, "", []string{"*.go"}, nil)
	_, err := commandScope.MutationExcludeFileArgv()
	if !errors.Is(err, ErrAllPackagesExcluded) {
		t.Fatalf("MutationExcludeFileArgv() err = %v, want ErrAllPackagesExcluded", err)
	}
}

func TestQualityScopeCommandScope_ProjectionParityWithoutSourceInventory(t *testing.T) {
	t.Parallel()
	commandScope := NewQualityScopeCommandScope(
		bddQualityScopeRows(), nil, "testutil", []string{"*_test.go"}, []string{"mage"},
	)
	if _, err := commandScope.CoverpkgCSV(); err != nil {
		t.Fatalf("CoverpkgCSV: %v", err)
	}
	if !commandScope.CoverageProfileFilter().Needed() {
		t.Fatal("expected coverage profile filtering")
	}
	if _, err := commandScope.GocycloPkgDirsRootRel(); err != nil {
		t.Fatalf("GocycloPkgDirsRootRel: %v", err)
	}
	exSegs, patterns := commandScope.ThresholdPathFilters()
	if len(exSegs) != 1 || exSegs[0] != "testutil" {
		t.Fatalf("exclude segments = %v", exSegs)
	}
	if len(patterns) != 1 || patterns[0] != "*_test.go" {
		t.Fatalf("patterns = %v", patterns)
	}
}

func TestMinimalCoverpkgCSV(t *testing.T) {
	t.Parallel()
	got, err := MinimalCoverpkgCSV(bddQualityScopeRows(), []string{"testutil"})
	if err != nil {
		t.Fatal(err)
	}
	want := "example.com/mod/zz_bdd_not_default/pkg"
	if got != want {
		t.Fatalf("MinimalCoverpkgCSV() = %q, want %q", got, want)
	}
}

func TestCoverageProfileFilter_Needed(t *testing.T) {
	t.Parallel()
	if !(CoverageProfileFilter{ExcludeSegments: []string{"x"}}).Needed() {
		t.Fatal("expected needed with exclude segment")
	}
	if !(CoverageProfileFilter{TestFilePatterns: []string{"*_test.go"}}).Needed() {
		t.Fatal("expected needed with test pattern")
	}
	if (CoverageProfileFilter{}).Needed() {
		t.Fatal("expected not needed for empty filter")
	}
}
