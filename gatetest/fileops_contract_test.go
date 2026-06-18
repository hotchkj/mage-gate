package gatetest_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
	"github.com/hotchkj/mage-gate/internal/fileopspath"
)

func mustRoot(tb testing.TB, m *gatetest.MemoryFileOps, root string) {
	tb.Helper()
	if err := m.Root(root); err != nil {
		tb.Fatal(err)
	}
}

func TestMemoryFileOps_RootRelativeReadWrite(t *testing.T) {
	t.Parallel()
	m := gatetest.NewMemoryFileOps()
	mustRoot(t, m, "/test-root")
	if err := m.WriteFile("artifacts/c.out", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := m.ReadFile("artifacts/c.out")
	if err != nil || string(b) != "x" {
		t.Fatalf("read %v %q", err, b)
	}
}

func TestMemoryFileOps_EmptyReadFileRejected(t *testing.T) {
	t.Parallel()
	m := gatetest.NewMemoryFileOps()
	mustRoot(t, m, "/test-root")
	_, err := m.ReadFile("")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, gatetest.ErrEmptyFileOpsPath) {
		t.Fatalf("want ErrEmptyFileOpsPath, got %v", err)
	}
}

func TestMemoryFileOps_UnrootedOperationsRejected(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	for name, run := range map[string]func() error{
		"mkdir": func() error { return mem.MkdirAll("x", 0o700) },
		"write": func() error { return mem.WriteFile("x.txt", []byte("x"), 0o600) },
		"walk":  func() error { return mem.Walk(".", func(string, fs.FileInfo, error) error { return nil }) },
	} {
		err := run()
		if err == nil {
			t.Fatalf("%s: expected unrooted error", name)
		}
		if !errors.Is(err, gatetest.ErrFileOpsNotRooted) {
			t.Fatalf("%s: got %v, want ErrFileOpsNotRooted", name, err)
		}
	}
}

func TestMemoryFileOps_MkdirTempEmptyAndDot(t *testing.T) {
	t.Parallel()
	m := gatetest.NewMemoryFileOps()
	mustRoot(t, m, "/test-root")
	for _, dir := range []string{"", "."} {
		t.Run("dir="+dir, func(t *testing.T) {
			t.Parallel()
			name, err := m.MkdirTemp(dir, "prefix-")
			if err != nil {
				t.Fatal(err)
			}
			if name == "" {
				t.Fatal("empty name")
			}
			if filepath.IsAbs(name) {
				t.Fatalf("non-canonical abs %q", name)
			}
		})
	}
}

func TestMemoryFileOps_MkdirTempBaseEscapes(t *testing.T) {
	t.Parallel()
	m := gatetest.NewMemoryFileOps()
	mustRoot(t, m, "/test-root")
	_, err := m.MkdirTemp("../outside", "x-")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fileopspath.ErrPathTraversal) {
		t.Fatalf("got %v", err)
	}
}

func TestMemoryFileOps_HostAbsoluteUnderRoot(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("use Windows-specific long-path test file for drive forms")
	}
	mem := gatetest.NewMemoryFileOps()
	const root = "/test-root"
	mustRoot(t, mem, root)
	if err := mem.WriteFile("inside/x.txt", []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(root, "inside", "x.txt")
	b, err := mem.ReadFile(abs)
	if err != nil || string(b) != "1" {
		t.Fatalf("%v %q", err, b)
	}
}

func TestMemoryFileOps_HostAbsoluteOutsideRoot(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, "/test-root")
	_, err := mem.ReadFile("/other-root/secret")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryFileOps_WalkDotRoot(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, "/test-root")
	if err := mem.WriteFile("a/b.go", []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	var seen bool
	if err := mem.Walk(".", func(path string, _ fs.FileInfo, _ error) error {
		if strings.HasSuffix(path, "a/b.go") {
			seen = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("walk missed file")
	}
}
