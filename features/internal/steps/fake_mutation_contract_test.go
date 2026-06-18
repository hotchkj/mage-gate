// Vision: BDD fakes write gremlins -o reports on canonical logical paths (same realm as FileOps reads).
package steps

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestMutationSitesFakeGremlinsUsesStepContractAnchorAndLogicalOutput(t *testing.T) {
	t.Parallel()

	// Mirrors [scenarioState.composeMutationSitesOpts]: gatetest.Gremlins(mem, ".", report).
	mem := gatetest.NewMemoryFileOps()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	report := []byte(`{"files":[{"file_name":"pkg/x.go","mutations":[]}]}`)
	wantPath := "mutation-scan-out.json"
	fr := cmdtest.NewFakeRunner(cmdtest.On("gremlins", gatetest.Gremlins(mem, ".", report)))
	err := fr.Run(
		context.Background(), ".", io.Discard, io.Discard,
		"gremlins", "unleash", "--dry-run", "--coverpkg=./...", "-o", wantPath,
	)
	if err != nil {
		t.Fatalf("fake gremlins unleash: %v", err)
	}
	got, readErr := mem.ReadFile(wantPath)
	if readErr != nil {
		t.Fatalf("read %q (expect canonical logical MemMapFs key): %v", wantPath, readErr)
	}
	if !bytes.Equal(got, report) {
		t.Fatalf("report mismatch")
	}
}

func TestMutationFakeGremlinsRejectsTraversalWithRootDot(t *testing.T) {
	t.Parallel()

	mem := gatetest.NewMemoryFileOps()
	g := gatetest.Gremlins(mem, ".", []byte(`{}`))
	cmd := cmdrunner.NewCommand(".", "gremlins", "unleash", "-o", "../evil.json")
	err := g(context.Background(), cmd, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, gatetest.ErrGremlinsReportPathEscape) {
		t.Fatalf("want ErrGremlinsReportPathEscape, got %v", err)
	}
}
