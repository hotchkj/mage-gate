// Vision: go test argument assembly: shuffle flags, coverpkg wiring, and package list edge cases.
package harness

import (
	"slices"
	"strings"
	"testing"
)

// exampleCoverprofileCommandPath stands in for [artifactPaths.CommandPath]("coverage.out") in tests of
// [buildTestArgs]. It matches [artifactPaths.FileOpsPath]("coverage.out") today but callers must treat
// the command-line projection independently.
const exampleCoverprofileCommandPath = "test-artifacts/coverage.out"

func TestBuildTestArgs_WithoutCoverpkg(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("./pkg", "", "", false, "", nil)
	want := []string{"test", "./pkg", "-json", "-count=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildTestArgs_WithCoverpkg(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("all", exampleCoverprofileCommandPath, "example.com/mod/...", false, "", nil)
	want := []string{
		"test", "all", "-json",
		"-coverprofile=test-artifacts/coverage.out",
		"-coverpkg=example.com/mod/...",
		"-count=1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildTestArgs_Short(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("p", "", "", true, "", nil)
	want := []string{"test", "p", "-json", "-short", "-count=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildTestArgs_Tags(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("p", "", "", false, "integration", nil)
	want := []string{"test", "p", "-json", "-tags=integration", "-count=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildTestArgs_MergesTags(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("p", "", "", false, "mage", []string{"-tags=integration", "-run", "TestFoo"})
	want := []string{"test", "p", "-json", "-tags=mage,integration", "-run", "TestFoo", "-count=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeBuildTags_SpaceSeparatedAndDoubleDashForms(t *testing.T) {
	t.Parallel()
	tags, filtered := mergeBuildTags("mage", []string{
		"--tags", "integration smoke",
		"--tags=slow",
		"-run", "TestFoo",
	})
	if tags != "mage,integration,smoke,slow" {
		t.Fatalf("tags = %q, want mage,integration,smoke,slow", tags)
	}
	wantFiltered := []string{"-run", "TestFoo"}
	if !slices.Equal(filtered, wantFiltered) {
		t.Fatalf("filtered = %v, want %v", filtered, wantFiltered)
	}
}

func TestMergeBuildTags_BareTagsFlagIsPreserved(t *testing.T) {
	t.Parallel()
	tags, filtered := mergeBuildTags("", []string{"-run", "TestFoo", "-tags"})
	if tags != "" {
		t.Fatalf("tags = %q, want empty", tags)
	}
	wantFiltered := []string{"-run", "TestFoo", "-tags"}
	if !slices.Equal(filtered, wantFiltered) {
		t.Fatalf("filtered = %v, want %v", filtered, wantFiltered)
	}
}

func TestBuildTestArgs_ExtraTestArgs(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("p", "", "", false, "", []string{"-run", "TestFoo"})
	want := []string{"test", "p", "-json", "-run", "TestFoo", "-count=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildTestArgs_Combined(t *testing.T) {
	t.Parallel()
	got := buildTestArgs("pkgs", exampleCoverprofileCommandPath, "x", true, "t", []string{"-parallel", "4"})
	want := []string{
		"test", "pkgs", "-json",
		"-coverprofile=test-artifacts/coverage.out",
		"-coverpkg=x",
		"-short",
		"-tags=t",
		"-parallel", "4",
		"-count=1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStripCountFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"empty slice", []string{}, []string{}},
		{"passthrough", []string{"-run", "TestX"}, []string{"-run", "TestX"}},
		{"equals form", []string{"-count=5", "-v"}, []string{"-v"}},
		{"double dash equals", []string{"--count=5", "-v"}, []string{"-v"}},
		{"space form", []string{"-count", "5", "-v"}, []string{"-v"}},
		{"double dash space", []string{"--count", "5", "-v"}, []string{"-v"}},
		{"bare count at end", []string{"-run", "X", "-count"}, []string{"-run", "X"}},
		{"bare double dash at end", []string{"--count"}, []string{}},
		{"mixed", []string{"-parallel", "2", "-count", "3", "-short"}, []string{"-parallel", "2", "-short"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripCountFlag(append([]string(nil), tc.in...))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestBuildTestArgs_HostileCountIsStripped(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testArgs []string
	}{
		{"equals form", []string{"-count=5"}},
		{"space form", []string{"-count", "5"}},
		{"double-dash equals", []string{"--count=5"}},
		{"double-dash space", []string{"--count", "5"}},
		{"mixed with other args", []string{"-run", "TestX", "-count=2", "-v"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildTestArgs("p", "", "", false, "", tc.testArgs)
			last := got[len(got)-1]
			if last != "-count=1" {
				t.Fatalf("last arg = %q, want -count=1; full args: %v", last, got)
			}
			for _, a := range got[:len(got)-1] {
				if strings.HasPrefix(a, "-count") || strings.HasPrefix(a, "--count") {
					t.Fatalf("unexpected count flag in non-final position: %q; full args: %v", a, got)
				}
			}
		})
	}
}
