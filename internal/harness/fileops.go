// Vision: Harness-local FileOps surface (mkdir, temp dirs, profile IO) abstracted for tests and production wiring.
package harness

import (
	"io"
	"io/fs"
	"path/filepath"
)

// FileOps abstracts rooted filesystem I/O for harness artifacts and source walks.
//
// Root must be called once before any other method. Implementations canonicalize
// path-like arguments at their boundary and reject traversal outside the root.
// Callers should pass canonical logical paths for artifact I/O; host-native paths
// are accepted only when containment proves they are under the configured root.
// Host path projection belongs at explicit native seams such as process execution
// or direct host filesystem adapters, not ordinary artifact reads or writes.
//
// Production wiring uses [gate.NewProductionFileOps] passed through gate step APIs.
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
