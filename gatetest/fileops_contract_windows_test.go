//go:build windows

package gatetest_test

import (
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func TestMemoryFileOps_WindowsLexicalDriveUnderRoot(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, `C:\proj`)
	const rel = `sub\a.go`
	if err := mem.WriteFile(rel, []byte("w"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := mem.ReadFile(`C:/proj/sub/a.go`)
	if err != nil || string(got) != "w" {
		t.Fatalf("read %v %q", err, got)
	}
}

func TestMemoryFileOps_WindowsDriveOutsideRoot(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	mustRoot(t, mem, `C:\proj`)
	_, err := mem.ReadFile(`D:\other\secret.go`)
	if err == nil {
		t.Fatal("expected error")
	}
}
