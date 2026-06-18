// Vision: StepRunner wiring—shared stores, sequential step IDs, and uniform cleanup/error wrapping for all steps.
package harness

import "github.com/hotchkj/mage-gate/cmdrunner"

const (
	defaultPackageScope = "./..."
	customLintBinary    = "custom-gcl"
	DefaultDeadcodeSpec = "golang.org/x/tools/cmd/deadcode@v0.31.0"
	artifactDirPerm     = 0o750
	artifactFilePerm    = 0o600
)

// StepRunner runs gate steps with shared working directory, artifact layout, and dependencies.
type StepRunner struct {
	root           string
	artifactSubdir string
	artifacts      artifactPaths
	packages       string
	runner         cmdrunner.CommandRunner
	fileOps        FileOps
	toolResolver   cmdrunner.ToolResolver
	store          ArtifactStore
	stepID         string
	tempOwned      bool
}

// ArtifactStore is an interface for storing and retrieving artifacts between steps.
type ArtifactStore interface {
	Write(stepID, name string, data []byte, prov Provenance) error
	Read(stepID, name string) ([]byte, error)
	Has(stepID, name string) bool
}

// NewStepRunner constructs a step runner. store must be non-nil ([NewDiscardArtifactStore] when no prior-step reads).
// Chained test → duration/coverage/CRAP/mutation passes share one store and a non-empty stepID (omit when unused).
// Empty artifactSubdir allocates an OS temp dir—call [StepRunner.Cleanup] to remove it; explicit dirs are left intact.
func NewStepRunner(
	root, artifactSubdir, packages string,
	runner cmdrunner.CommandRunner,
	fileOps FileOps,
	store ArtifactStore,
	stepID string,
	opts ...StepRunnerOption,
) (*StepRunner, error) {
	if err := validateStepRunnerInputs(root, runner, fileOps, store); err != nil {
		return nil, err
	}
	if rootErr := fileOps.Root(root); rootErr != nil {
		return nil, rootErr
	}
	harn := &StepRunner{
		root:           root,
		artifactSubdir: artifactSubdir,
		packages:       packages,
		runner:         runner,
		fileOps:        fileOps,
		store:          store,
		stepID:         stepID,
	}
	for _, opt := range opts {
		opt(harn)
	}
	if err := harn.ensureArtifactSubdir(); err != nil {
		return nil, err
	}
	apaths, err := newArtifactPaths(harn.root, harn.artifactSubdir)
	if err != nil {
		return nil, err
	}
	harn.artifacts = apaths
	return harn, nil
}

func validateStepRunnerInputs(
	root string,
	runner cmdrunner.CommandRunner,
	fileOps FileOps,
	store ArtifactStore,
) error {
	if root == "" {
		return ErrRootRequired
	}
	if store == nil {
		return ErrArtifactStoreRequired
	}
	if runner == nil || fileOps == nil {
		return ErrDepsRequired
	}
	return nil
}

func (h *StepRunner) ensureArtifactSubdir() error {
	if h.artifactSubdir != "" {
		return nil
	}
	h.tempOwned = true
	tempDir, err := h.fileOps.MkdirTemp("", "mage-gate-")
	if err != nil {
		return err
	}
	h.artifactSubdir = tempDir
	return nil
}

// Cleanup removes the temporary artifact directory when NewStepRunner created it.
func (h *StepRunner) Cleanup() error {
	if !h.tempOwned {
		return nil
	}
	return h.fileOps.RemoveAll(h.artifactSubdir)
}

func resolvePackages(packages string) string {
	if packages == "" {
		return defaultPackageScope
	}
	return packages
}
