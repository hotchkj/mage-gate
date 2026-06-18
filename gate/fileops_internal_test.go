// Vision: production FileOps path translation covered without touching the real filesystem.
package gate

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/spf13/afero"
)

func newMemoryBackedProductionFileOps() *productionFileOps {
	return &productionFileOps{fs: afero.NewMemMapFs(), rootArg: "/gate-root"}
}

func TestProductionFileOpsMemoryBackedReadWriteAndCreate(t *testing.T) {
	t.Parallel()
	fileOps := newMemoryBackedProductionFileOps()
	if err := fileOps.MkdirAll(`artifacts\sub`, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fileOps.WriteFile(`artifacts\sub\data.txt`, []byte("one"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	absUnderRoot := "/gate-root/artifacts/sub/data.txt"
	got, err := fileOps.ReadFile(absUnderRoot)
	if err != nil {
		t.Fatalf("ReadFile host-absolute under root: %v", err)
	}
	if string(got) != "one" {
		t.Fatalf("ReadFile = %q, want one", got)
	}
	writer, err := fileOps.CreateFile("artifacts/sub/created.txt")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := io.WriteString(writer, "created"); err != nil {
		t.Fatalf("write created file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close created file: %v", err)
	}
}

func TestProductionFileOpsMemoryBackedWalkDisplaysRootedPaths(t *testing.T) {
	t.Parallel()
	fileOps := newMemoryBackedProductionFileOps()
	if err := fileOps.WriteFile("artifacts/sub/created.txt", []byte("created"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var sawCreated bool
	if err := fileOps.Walk(".", func(path string, _ fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.HasSuffix(filepath.ToSlash(path), "artifacts/sub/created.txt") {
			sawCreated = true
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !sawCreated {
		t.Fatal("Walk did not visit created file")
	}
}

func TestProductionFileOpsMemoryBackedMkdirTempAndRemoveAll(t *testing.T) {
	t.Parallel()
	fileOps := newMemoryBackedProductionFileOps()
	name, err := fileOps.MkdirTemp("", "tmp-")
	if err != nil {
		t.Fatalf("MkdirTemp empty base: %v", err)
	}
	if filepath.IsAbs(name) {
		t.Fatalf("MkdirTemp returned host-absolute path %q", name)
	}
	if err := fileOps.WriteFile(filepath.Join(name, "x.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile temp child: %v", err)
	}
	if err := fileOps.RemoveAll(name); err != nil {
		t.Fatalf("RemoveAll temp dir: %v", err)
	}
	if _, err := fileOps.ReadFile(filepath.Join(name, "x.txt")); err == nil {
		t.Fatal("expected removed temp child to be unreadable")
	}
}

func TestProductionFileOpsMemoryBackedRejectsTraversal(t *testing.T) {
	t.Parallel()
	fileOps := newMemoryBackedProductionFileOps()
	for name, run := range map[string]func() error{
		"mkdir": func() error { return fileOps.MkdirAll("../outside", 0o700) },
		"temp": func() error {
			_, err := fileOps.MkdirTemp("../outside", "x-")
			return err
		},
		"read": func() error {
			_, err := fileOps.ReadFile("../outside.txt")
			return err
		},
		"walk": func() error {
			return fileOps.Walk("../outside", func(string, fs.FileInfo, error) error { return nil })
		},
	} {
		err := run()
		if err == nil {
			t.Fatalf("%s: expected traversal error", name)
		}
		if !errors.Is(err, fileopspath.ErrPathTraversal) {
			t.Fatalf("%s: got %v, want ErrPathTraversal", name, err)
		}
	}
}
