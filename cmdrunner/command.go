// Vision: Command value object—OS-native cwd resolution, defensive argv/env copies, safe accessors for exec.
package cmdrunner

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// Command captures a subprocess invocation. Fields are unexported to enforce
// construction through NewCommand, which normalizes Dir for exec.Cmd and copies Args.
type Command struct {
	name string
	args []string
	dir  string
}

// DirNativeForExec returns a filepath.Clean OS-native working directory string suitable
// for exec.Cmd.Dir. It applies fsnorm.Canonical first so mixed separators normalize
// consistently, then filepath.FromSlash so Windows receives backslashes where expected.
// Empty input becomes ".". Use fsnorm.Canonical(dir) when comparing cwd across GOOS in tests.
func DirNativeForExec(dir string) string {
	if isWindowsExtendedPath(dir) {
		return filepath.Clean(dir)
	}
	c := fsnorm.Canonical(dir)
	if c == "" {
		c = "."
	}
	return filepath.Clean(filepath.FromSlash(c))
}

func isWindowsExtendedPath(dir string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return strings.HasPrefix(dir, `\\?\`) || strings.HasPrefix(dir, `//?/`)
}

// NewCommand creates a Command with a defensively copied args slice and dir set to
// DirNativeForExec(dir) for subprocess execution.
func NewCommand(dir, name string, args ...string) Command {
	copied := make([]string, len(args))
	copy(copied, args)
	return Command{
		name: name,
		args: copied,
		dir:  DirNativeForExec(dir),
	}
}

// Name returns the binary name (e.g. "go", "golangci-lint").
func (c Command) Name() string { return c.name }

// Args returns a copy of the argument slice. Callers cannot alias internal state.
func (c Command) Args() []string {
	out := make([]string, len(c.args))
	copy(out, c.args)
	return out
}

// Dir returns the OS-native working directory for exec ("." for CWD). For lexical
// comparison across platforms (logs, tests), use fsnorm.Canonical(c.Dir()).
func (c Command) Dir() string { return c.dir }

// Arg returns args[i] or "" if i is out of bounds.
func (c Command) Arg(i int) string {
	if i < 0 || i >= len(c.args) {
		return ""
	}
	return c.args[i]
}
