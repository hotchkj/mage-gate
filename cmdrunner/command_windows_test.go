package cmdrunner_test

import (
	"path/filepath"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

func TestDirNativeForExec_PreservesWindowsExtendedPathPrefix(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{
		`\\?\C:\very-long-root\pkg\..\module`,
		`//?/C:/very-long-root/pkg/../module`,
	} {
		want := filepath.Clean(dir)
		if got := cmdrunner.DirNativeForExec(dir); got != want {
			t.Fatalf("DirNativeForExec(%q) = %q, want %q", dir, got, want)
		}
	}
}
