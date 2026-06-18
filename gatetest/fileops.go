// Vision: Memory-backed FileOps for gate/harness tests without pulling test doubles into production packages.
package gatetest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
	"github.com/spf13/afero"
)

// ErrEmptyFileOpsPath indicates a file-oriented operation rejected an empty or
// directory-only logical path after canonicalization.
var (
	ErrEmptyFileOpsPath = errors.New("empty file path")
	ErrFileOpsNotRooted = errors.New("memory fileops Root must succeed before filesystem operations")
	errEmptyFileOpsRoot = errors.New("empty fileops root")
)

// MemoryFileOps wraps afero.MemMapFs to satisfy the gate.FileOps contract.
type MemoryFileOps struct {
	fs afero.Fs

	rooted bool
	// rootArg is the verbatim clean root argument from Root (same contract as harness StepRunner.root).
	rootArg string
}

// NewMemoryFileOps returns a fresh unbound memory FileOps. Call [MemoryFileOps.Root]
// before filesystem operations; harness configures Root during construction.
//
// Concrete return type avoids importing gate (gate tests already import this package).
func NewMemoryFileOps() *MemoryFileOps {
	return &MemoryFileOps{fs: afero.NewMemMapFs()}
}

// Fork returns a fresh unbound FileOps handle over the same in-memory filesystem.
// Root the fork before use; this lets tests share MemMapFs state without sharing
// a root binding.
func (m *MemoryFileOps) Fork() *MemoryFileOps {
	return &MemoryFileOps{fs: m.fs}
}

// Root configures the MemMapFs logical containment root. Repeating Root with the same
// filepath.Clean value is idempotent; a different non-empty root after the first
// Root returns [fileopspath.ErrFileOpsRootAlreadyBound].
func (m *MemoryFileOps) Root(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: memory fileops Root", errEmptyFileOpsRoot)
	}
	cr := filepath.Clean(strings.TrimSpace(root))
	if m.rooted {
		if filepath.Clean(m.rootArg) == cr {
			return nil
		}
		return fileopspath.ErrFileOpsRootAlreadyBound
	}
	m.rootArg = cr
	m.rooted = true
	return nil
}

func (m *MemoryFileOps) mapDir(raw string) (logical string, err error) {
	if !m.rooted {
		return "", ErrFileOpsNotRooted
	}

	dirInput := raw
	if strings.TrimSpace(dirInput) == "" {
		dirInput = "."
	}
	out, terr := fileopspath.LogicalContainedRelative(m.rootArg, dirInput)
	if terr != nil {
		return "", fmt.Errorf("%w: %w", fileopspath.ErrPathTraversal, terr)
	}
	return out, nil
}

func (m *MemoryFileOps) mapFile(raw string) (logical string, err error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrEmptyFileOpsPath
	}
	canon := fsnorm.Canonical(raw)
	if canon == "" || canon == "." {
		return "", ErrEmptyFileOpsPath
	}
	if !m.rooted {
		return "", ErrFileOpsNotRooted
	}
	out, terr := fileopspath.LogicalContainedRelative(m.rootArg, raw)
	if terr != nil {
		return "", fmt.Errorf("%w: %w", fileopspath.ErrPathTraversal, terr)
	}
	if out == "." || out == "" {
		return "", ErrEmptyFileOpsPath
	}
	return out, nil
}

func (m *MemoryFileOps) MkdirAll(path string, perm fs.FileMode) error {
	lp, err := m.mapDir(path)
	if err != nil {
		return err
	}
	return m.fs.MkdirAll(lp, perm)
}

// MkdirTemp keeps MemMapFs temp directories rooted at "." internally; callers receive canonical logical paths.
func (m *MemoryFileOps) MkdirTemp(dir, pattern string) (string, error) {
	dirInput := dir
	if strings.TrimSpace(dirInput) == "" {
		dirInput = "."
	}
	base, err := m.mapDir(dirInput)
	if err != nil {
		return "", err
	}
	if base == "" {
		base = "."
	}
	name, err := afero.TempDir(m.fs, base, pattern)
	if err != nil {
		return "", err
	}
	return fsnorm.Canonical(filepath.ToSlash(name)), nil
}

func (m *MemoryFileOps) RemoveAll(path string) error {
	lp, err := m.mapDir(path)
	if err != nil {
		return err
	}
	return m.fs.RemoveAll(lp)
}

func (m *MemoryFileOps) WriteFile(path string, data []byte, perm fs.FileMode) error {
	lp, err := m.mapFile(path)
	if err != nil {
		return err
	}
	return afero.WriteFile(m.fs, lp, data, perm)
}

func (m *MemoryFileOps) ReadFile(path string) ([]byte, error) {
	lp, err := m.mapFile(path)
	if err != nil {
		return nil, err
	}
	return afero.ReadFile(m.fs, lp)
}

func (m *MemoryFileOps) CreateFile(path string) (io.WriteCloser, error) {
	lp, err := m.mapFile(path)
	if err != nil {
		return nil, err
	}
	return m.fs.Create(lp)
}

func (m *MemoryFileOps) Walk(root string, fn filepath.WalkFunc) error {
	if !m.rooted {
		return ErrFileOpsNotRooted
	}
	interiorCanon, err := fileopspath.LogicalContainedRelative(m.rootArg, root)
	if err != nil {
		return fmt.Errorf("%w: walk root: %w", fileopspath.ErrPathTraversal, err)
	}
	displayCanon := fsnorm.Canonical(m.rootArg)
	return afero.Walk(m.fs, interiorCanon, func(path string, info fs.FileInfo, walkErr error) error {
		cb := path
		if walkErr == nil && info != nil {
			cb = fileopspath.DisplayWalkPath(displayCanon, interiorCanon, path)
		}
		return fn(cb, info, walkErr)
	})
}
