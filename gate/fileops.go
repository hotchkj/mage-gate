// Vision: FileOps boundary for gate callers—injects disk or fakes without importing harness internals.
// Production wiring: [NewProductionFileOps]; [productionFileOps.Root] is invoked by harness.StepRunner before IO.
package gate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/hotchkj/mage-gate/internal/harness"
	"github.com/spf13/afero"
)

const filePermDefault = 0o600

var (
	errEmptyFileOpsPath = errors.New("empty file path")
	errEmptyFileOpsRoot = errors.New("empty fileops root")
	errFileOpsNotRooted = errors.New("FileOps.Root must succeed before filesystem operations")
)

// FileOps matches harness.FileOps structurally—tests use gatetest memory fakes; production uses [NewProductionFileOps].
//
// Path realm: methods accept path-like arguments in relative, mixed-separator, or (when contained) host-absolute
// form; each implementation canonicalizes and enforces containment under the pinned root before touching the rooted
// afero layer. Returned paths (for example [productionFileOps.MkdirTemp]) are canonical logical (forward slashes) for
// inventories and comparisons. Subprocess command cwd ([exec.Cmd.Dir]) stays OS-native and is wired in cmdrunner (gate
// root unless documented otherwise); this interface does not replace that boundary.
type FileOps interface {
	Root(root string) error
	MkdirAll(path string, perm fs.FileMode) error
	MkdirTemp(dir, pattern string) (string, error)
	RemoveAll(path string) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadFile(path string) ([]byte, error)
	CreateFile(path string) (io.WriteCloser, error)
	Walk(root string, fn filepath.WalkFunc) error
}

type productionFileOps struct {
	fs      afero.Fs // rooted BasePathFs; nil until [productionFileOps.Root]
	rootArg string   // verbatim host root argument for containment (same tier as harness root)
}

// Compile-time check: gate.FileOps stays aligned with harness.FileOps for DI.
var _ harness.FileOps = (*productionFileOps)(nil)

// Root pins the rooted OsFs/BasePathFs namespace. Calling Root again with the same
// filepath.Clean(strings.TrimSpace(root)) value is idempotent; a different root after the first
// successful Root returns internal/fileopspath.ErrFileOpsRootAlreadyBound (wrapped).
func (p *productionFileOps) Root(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: fileops Root", errEmptyFileOpsRoot)
	}
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if p.fs != nil {
		if filepath.Clean(p.rootArg) == cleanRoot {
			return nil
		}
		return fmt.Errorf("%w: was %q, got %q", fileopspath.ErrFileOpsRootAlreadyBound, p.rootArg, cleanRoot)
	}
	abs, err := filepath.Abs(cleanRoot)
	if err != nil {
		return fmt.Errorf("fileops Root: %w", err)
	}
	p.rootArg = cleanRoot
	p.fs = afero.NewBasePathFs(afero.NewOsFs(), abs)
	return nil
}

func (p *productionFileOps) requireFS() error {
	if p.fs == nil {
		return errFileOpsNotRooted
	}
	return nil
}

func canonicalFileOpsPathInput(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errEmptyFileOpsPath
	}
	c := fsnorm.Canonical(raw)
	if c == "" || c == "." {
		return "", errEmptyFileOpsPath
	}
	return c, nil
}

func (p *productionFileOps) translateDirRaw(raw string) (string, error) {
	if fsErr := p.requireFS(); fsErr != nil {
		return "", fsErr
	}
	dir := raw
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	logicalRel, err := fileopspath.LogicalContainedRelative(p.rootArg, dir)
	if err != nil {
		return "", fmt.Errorf("%w: %w", fileopspath.ErrPathTraversal, err)
	}
	return filepath.FromSlash(logicalRel), nil
}

func (p *productionFileOps) translateFileRaw(raw string) (string, error) {
	if _, inputErr := canonicalFileOpsPathInput(raw); inputErr != nil {
		return "", inputErr
	}
	if fsErr := p.requireFS(); fsErr != nil {
		return "", fsErr
	}
	logicalRel, err := fileopspath.LogicalContainedRelative(p.rootArg, raw)
	if err != nil {
		return "", fmt.Errorf("%w: %w", fileopspath.ErrPathTraversal, err)
	}
	if logicalRel == "." || logicalRel == "" {
		return "", errEmptyFileOpsPath
	}
	return filepath.FromSlash(logicalRel), nil
}

func (p *productionFileOps) MkdirAll(path string, perm fs.FileMode) error {
	if err := p.requireFS(); err != nil {
		return err
	}
	rel, err := p.translateDirRaw(path)
	if err != nil {
		return err
	}
	return p.fs.MkdirAll(rel, perm)
}

func (p *productionFileOps) MkdirTemp(dir, pattern string) (string, error) {
	if err := p.requireFS(); err != nil {
		return "", err
	}
	dirInput := dir
	if strings.TrimSpace(dirInput) == "" {
		dirInput = "."
	}
	baseLogical, err := fileopspath.LogicalContainedRelative(p.rootArg, dirInput)
	if err != nil {
		return "", fmt.Errorf("%w: mkdir temp base: %w", fileopspath.ErrPathTraversal, err)
	}
	if baseLogical == "" {
		baseLogical = "."
	}
	name, err := afero.TempDir(p.fs, filepath.FromSlash(baseLogical), pattern)
	if err != nil {
		return "", err
	}
	return fsnorm.Canonical(filepath.ToSlash(name)), nil
}

func (p *productionFileOps) RemoveAll(path string) error {
	if err := p.requireFS(); err != nil {
		return err
	}
	rel, err := p.translateDirRaw(path)
	if err != nil {
		return err
	}
	return p.fs.RemoveAll(rel)
}

func (p *productionFileOps) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if err := p.requireFS(); err != nil {
		return err
	}
	rel, err := p.translateFileRaw(path)
	if err != nil {
		return err
	}
	return afero.WriteFile(p.fs, rel, data, perm)
}

func (p *productionFileOps) ReadFile(path string) ([]byte, error) {
	if err := p.requireFS(); err != nil {
		return nil, err
	}
	rel, err := p.translateFileRaw(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 — path is routed through rooted BasePathFs after containment checks.
	return afero.ReadFile(p.fs, rel)
}

func (p *productionFileOps) CreateFile(path string) (io.WriteCloser, error) {
	if err := p.requireFS(); err != nil {
		return nil, err
	}
	rel, err := p.translateFileRaw(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 — path is routed through rooted BasePathFs after containment checks.
	return p.fs.OpenFile(rel, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermDefault)
}

func (p *productionFileOps) Walk(root string, fn filepath.WalkFunc) error {
	if err := p.requireFS(); err != nil {
		return err
	}
	interiorCanon, err := fileopspath.LogicalContainedRelative(p.rootArg, root)
	if err != nil {
		return fmt.Errorf("%w: walk root: %w", fileopspath.ErrPathTraversal, err)
	}
	displayCanon := fsnorm.Canonical(p.rootArg)
	innerWalk := filepath.FromSlash(interiorCanon)

	return afero.Walk(p.fs, innerWalk, func(path string, info fs.FileInfo, walkErr error) error {
		callPath := path
		if walkErr == nil && info != nil {
			callPath = fileopspath.DisplayWalkPath(displayCanon, interiorCanon, path)
		}
		return fn(callPath, info, walkErr)
	})
}

// NewProductionFileOps returns production FileOps: configure with [productionFileOps.Root] before filesystem methods;
// harness StepRunner calls Root during construction.
func NewProductionFileOps() FileOps {
	return &productionFileOps{}
}
