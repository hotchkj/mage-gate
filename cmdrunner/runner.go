// Vision: Production CommandRunner—context-aware exec, working-directory hygiene, and streamed stdout/stderr.
package cmdrunner

import (
	"context"
	"io"
	"os/exec"
)

// assignExecWorkingDir sets cmd.Dir to [DirNativeForExec] — the same cwd normalization recorded by
// [NewCommand], so FakeRunner-produced commands and subprocess execution share cwd semantics.
func assignExecWorkingDir(cmd *exec.Cmd, dir string) {
	if cmd == nil {
		return
	}
	cmd.Dir = DirNativeForExec(dir)
}

// CommandRunner is an interface for running external commands.
type CommandRunner interface {
	Run(ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string) error
}

type execRunner struct{}

func (e *execRunner) Run(ctx context.Context, dir string, stdout, stderr io.Writer, name string, args ...string) error {
	// #nosec G204 -- build-tool runner: name is the literal "go", "golangci-lint", or an
	// artifact-dir binary path. Config-derived args (gate.toml) share the same trust boundary
	// as the source code being built — modifying config requires repo write access, which
	// already grants arbitrary code execution via source edits.
	cmd := exec.CommandContext(ctx, name, args...)
	assignExecWorkingDir(cmd, dir)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

var _ CommandRunner = (*execRunner)(nil)

// NewProductionRunner returns a CommandRunner that executes real OS commands.
func NewProductionRunner() CommandRunner {
	return &execRunner{}
}
