// Vision: Dogfood-shaped gremlins exclude tests proving real inventory + walked source + segments minimize correctly.
package gatecheck

import (
	"reflect"
	"strings"
	"testing"
)

func dogfoodTestPatternRows() []MutationPackageRow {
	return []MutationPackageRow{
		{
			ImportPath: "github.com/hotchkj/mage-gate/cmdrunner", PkgDirRootRel: "cmdrunner",
			GoFileNames: []string{"run.go"}, TestGoFileNames: []string{"run_test.go", "extra_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/gate", PkgDirRootRel: "gate",
			GoFileNames: []string{"gate.go"}, TestGoFileNames: []string{"a_test.go", "b_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/fileopspath", PkgDirRootRel: "internal/fileopspath",
			GoFileNames: []string{"p.go"}, TestGoFileNames: []string{"p_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/fsnorm", PkgDirRootRel: "internal/fsnorm",
			GoFileNames: []string{"n.go"}, TestGoFileNames: []string{"n_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/gatecheck", PkgDirRootRel: "internal/gatecheck",
			GoFileNames: []string{"g.go"}, TestGoFileNames: []string{"g_test.go", "h_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/harness", PkgDirRootRel: "internal/harness",
			GoFileNames: []string{"h.go"}, TestGoFileNames: []string{"h_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/magefiles", PkgDirRootRel: "magefiles",
			GoFileNames: []string{"m.go"}, TestGoFileNames: []string{"m_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/cmdtest", PkgDirRootRel: "cmdtest",
			GoFileNames: []string{"t.go"}, TestGoFileNames: []string{"t_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/testdata", PkgDirRootRel: "testdata",
			GoFileNames: []string{"d.go"}, TestGoFileNames: []string{"d_test.go"},
		},
	}
}

func TestBuildGremlinsExcludeArgv_dogfoodTestPatternRegexes(t *testing.T) {
	t.Parallel()
	excludeSegments := []string{"cmdtest", "features", "gatetest", "integration", "testdata"}
	got, err := BuildGremlinsExcludeArgv(dogfoodTestPatternRows(), nil, excludeSegments, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^cmdrunner/.*_test\.go$`,
		`--exclude-files=^cmdtest(/|$)`,
		`--exclude-files=^gate/.*_test\.go$`,
		`--exclude-files=^internal/fileopspath/.*_test\.go$`,
		`--exclude-files=^internal/fsnorm/.*_test\.go$`,
		`--exclude-files=^internal/gatecheck/.*_test\.go$`,
		`--exclude-files=^internal/harness/.*_test\.go$`,
		`--exclude-files=^magefiles/.*_test\.go$`,
		`--exclude-files=^testdata(/|$)`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func dogfoodShapeRows() []MutationPackageRow {
	return []MutationPackageRow{
		{
			ImportPath: "github.com/hotchkj/mage-gate/cmdrunner", PkgDirRootRel: "cmdrunner",
			GoFileNames: []string{"run.go"}, TestGoFileNames: []string{"run_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/gate", PkgDirRootRel: "gate",
			GoFileNames: []string{"gate.go"}, TestGoFileNames: []string{"a_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/fileopspath", PkgDirRootRel: "internal/fileopspath",
			GoFileNames: []string{"p.go"}, TestGoFileNames: []string{"p_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/fsnorm", PkgDirRootRel: "internal/fsnorm",
			GoFileNames: []string{"n.go"}, TestGoFileNames: []string{"n_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/gatecheck", PkgDirRootRel: "internal/gatecheck",
			GoFileNames: []string{"g.go"}, TestGoFileNames: []string{"g_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/internal/harness", PkgDirRootRel: "internal/harness",
			GoFileNames: []string{"h.go"}, TestGoFileNames: []string{"h_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/magefiles", PkgDirRootRel: "magefiles",
			GoFileNames: []string{"m.go"}, TestGoFileNames: []string{"m_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/cmdtest", PkgDirRootRel: "cmdtest",
			GoFileNames: []string{"t.go"}, TestGoFileNames: []string{"t_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/features", PkgDirRootRel: "features",
			GoFileNames: []string{"f.go"}, TestGoFileNames: []string{"f_test.go"},
		},
		{
			ImportPath: "github.com/hotchkj/mage-gate/gatetest", PkgDirRootRel: "gatetest",
			GoFileNames: []string{"g.go"}, TestGoFileNames: []string{"g_test.go"},
		},
	}
}

func TestBuildGremlinsExcludeArgv_dogfoodShapeNoSyntheticRows(t *testing.T) {
	t.Parallel()
	sourceFiles := []string{
		"cmdrunner/run.go", "cmdtest/t.go", "features/f.go", "gate/gate.go",
		"gatetest/g.go", "integration/examplecheck/integration_test.go",
		"internal/fileopspath/p.go", "internal/fsnorm/n.go",
		"internal/gatecheck/g.go", "internal/harness/h.go",
		"magefiles/m.go", "testdata/failures/calc.go", "testdata/failures/other.go",
	}
	excludeSegments := []string{"cmdtest", "features", "gatetest", "integration", "testdata"}
	got, err := BuildGremlinsExcludeArgv(dogfoodShapeRows(), sourceFiles, excludeSegments, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`--exclude-files=^cmdrunner/.*_test\.go$`,
		`--exclude-files=^cmdtest(/|$)`,
		`--exclude-files=^features(/|$)`,
		`--exclude-files=^gate/.*_test\.go$`,
		`--exclude-files=^gatetest(/|$)`,
		`--exclude-files=^integration(/|$)`,
		`--exclude-files=^internal/fileopspath/.*_test\.go$`,
		`--exclude-files=^internal/fsnorm/.*_test\.go$`,
		`--exclude-files=^internal/gatecheck/.*_test\.go$`,
		`--exclude-files=^internal/harness/.*_test\.go$`,
		`--exclude-files=^magefiles/.*_test\.go$`,
		`--exclude-files=^testdata(/|$)`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	assertNoPerFileExcludesForSegments(t, got, "testdata/", "integration/")
}

func assertNoPerFileExcludesForSegments(t *testing.T, flags []string, segments ...string) {
	t.Helper()
	for _, flag := range flags {
		for _, seg := range segments {
			if strings.HasPrefix(flag, "--exclude-files=^"+seg) && !strings.HasSuffix(flag, "(/|$)") {
				t.Fatalf("per-file exclude found for segment %q: %s", seg, flag)
			}
		}
	}
}
