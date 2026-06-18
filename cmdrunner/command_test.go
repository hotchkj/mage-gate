// Vision: Command value invariants: cwd normalization, argv copies, and safe read-only accessors.
package cmdrunner_test

import (
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

const testArgTest = "test"

func TestDirNativeForExec_AlignsWithNewCommand(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{".", "", "x/y", `a\b\c`, "a/../b"} {
		cmd := cmdrunner.NewCommand(dir, "go")
		if w, g := cmdrunner.DirNativeForExec(dir), cmd.Dir(); g != w {
			t.Fatalf("dir %q: Dir()=%q DirNativeForExec=%q", dir, g, w)
		}
	}
}

func TestNewCommand_DirCleaned(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dir     string
		wantDir string
	}{
		{"empty becomes dot", "", "."},
		{"dot stays dot", ".", "."},
		{"relative cleaned", "a/../b", "b"},
		{
			"absolute cleaned",
			filepath.Join(string(filepath.Separator), "a", "..", "b"),
			cmdrunner.DirNativeForExec(filepath.Join(string(filepath.Separator), "a", "..", "b")),
		},
		{"trailing sep removed", "foo" + string(filepath.Separator), "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := cmdrunner.NewCommand(tt.dir, "go", "test")
			if cmd.Dir() != tt.wantDir {
				t.Fatalf("Dir() = %q, want %q", cmd.Dir(), tt.wantDir)
			}
		})
	}
}

func TestNewCommand_ArgsCopied(t *testing.T) {
	t.Parallel()
	original := []string{"test", "./..."}
	cmd := cmdrunner.NewCommand(".", "go", original...)
	original[0] = "mutated"
	if cmd.Arg(0) != testArgTest {
		t.Fatalf("mutation of input affected Command: got %q", cmd.Arg(0))
	}
}

func TestCommand_ArgsReturnsCopy(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...")
	args := cmd.Args()
	args[0] = "mutated"
	if cmd.Arg(0) != testArgTest {
		t.Fatalf("mutation of Args() return affected Command: got %q", cmd.Arg(0))
	}
}

func TestCommand_NameAndDir(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand("/project", "golangci-lint", "run")
	if cmd.Name() != "golangci-lint" {
		t.Fatalf("Name() = %q", cmd.Name())
	}
	want := cmdrunner.DirNativeForExec("/project")
	if cmd.Dir() != want {
		t.Fatalf("Dir() = %q, want %q", cmd.Dir(), want)
	}
}

func TestCommand_Arg_InBounds(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-json", "./...")
	if cmd.Arg(0) != testArgTest {
		t.Fatalf("Arg(0) = %q", cmd.Arg(0))
	}
	if cmd.Arg(2) != "./..." {
		t.Fatalf("Arg(2) = %q", cmd.Arg(2))
	}
}

func TestCommand_Arg_OutOfBounds(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test")
	if cmd.Arg(-1) != "" {
		t.Fatalf("Arg(-1) = %q, want empty", cmd.Arg(-1))
	}
	if cmd.Arg(99) != "" {
		t.Fatalf("Arg(99) = %q, want empty", cmd.Arg(99))
	}
}

func TestCommand_Arg_EmptyArgs(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go")
	if cmd.Arg(0) != "" {
		t.Fatalf("Arg(0) on empty args = %q, want empty", cmd.Arg(0))
	}
}
