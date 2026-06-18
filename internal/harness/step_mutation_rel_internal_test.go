// Vision: Strict module-root-relative path helpers for gremlins excludes.
package harness

import (
	"testing"
)

func TestPackageDirModuleRelative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		moduleRoot string
		pkgDir     string
		wantRel    string
	}{
		{
			name:       "pkg_equals_module",
			moduleRoot: "/mod",
			pkgDir:     "/mod",
			wantRel:    ".",
		},
		{
			name:       "nested_under_module_unix",
			moduleRoot: "/mod",
			pkgDir:     "/mod/internal/app",
			wantRel:    "internal/app",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := packageDirModuleRelative(tc.moduleRoot, tc.pkgDir)
			if err != nil {
				t.Fatalf("packageDirModuleRelative: %v", err)
			}
			if got != tc.wantRel {
				t.Fatalf("packageDirModuleRelative(%q, %q) = %q, want %q",
					tc.moduleRoot, tc.pkgDir, got, tc.wantRel)
			}
		})
	}
}

func TestParseGoListMutationLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		line string
		want mutationListLineFields
	}{
		{name: "six_col", line: "p\t/dir/p\t\t\t/mod\tfoo.go;bar.go", want: mutationListLineFields{
			imp: "p", pkgDir: "/dir/p", modCol: "/mod", goCol: "foo.go;bar.go",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseGoListMutationLine(tc.line)
			if err != nil {
				t.Fatalf("parseGoListMutationLine: %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseGoListMutationLine = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseGoListMutationLineRejectsBadFieldCount(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"only", "p\t/dir/p", "p\t/dir/p\t\t\t/mod"} {
		if _, err := parseGoListMutationLine(line); err == nil {
			t.Fatalf("expected err for bad field count %q", line)
		}
	}
}

func TestPkgDirRootRelForMutationRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root string
		mod  string
		pkg  string
		want string
	}{
		{name: "module relative", root: "/test-root", mod: "/mod", pkg: "/mod/p", want: "p"},
		{name: "root relative", root: "/test-root", pkg: "/test-root/internal/app", want: "internal/app"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pkgDirRootRelForMutationRow(tc.root, tc.mod, tc.pkg)
			if err != nil {
				t.Fatalf("pkgDirRootRelForMutationRow: %v", err)
			}
			if got != tc.want {
				t.Fatalf("pkgDirRootRelForMutationRow = %q, want %q", got, tc.want)
			}
		})
	}
}
