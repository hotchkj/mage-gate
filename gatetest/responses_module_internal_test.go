// Vision: GoList fake responses stay aligned with the module-path queries the harness issues during setup.
package gatetest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

func TestGoList_modulePathQueryMatchesHarness(t *testing.T) {
	t.Parallel()
	h := GoList("example.com/mod", "/mod", map[string]PackageListInfo{
		"example.com/mod/pkg": DirOnly("/mod/pkg"),
	})
	cmd := cmdrunner.NewCommand(".", "go", "list", "-m", "-f", "{{.Path}}")
	var stdout strings.Builder
	if err := h(context.Background(), cmd, &stdout, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "example.com/mod" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestGoList_moduleJSONEmitsSingleObjectWithPathAndDir(t *testing.T) {
	t.Parallel()
	fakeGoList := GoList("example.com/mod", "/mod", map[string]PackageListInfo{
		"example.com/mod/pkg": DirOnly("/mod/pkg"),
	})
	cmd := cmdrunner.NewCommand(".", "go", "list", "-m", "-json")
	var stdout strings.Builder
	if err := fakeGoList(context.Background(), cmd, &stdout, io.Discard); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(stdout.String()))
	var got struct {
		Path string
		Dir  string
	}
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decode module JSON: %v", err)
	}
	if got.Path != "example.com/mod" || got.Dir != "/mod" {
		t.Fatalf("module JSON = %#v, want Path and Dir", got)
	}
	var extra struct {
		Path string
		Dir  string
	}
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("expected one module JSON object, got extra decode err %v and value %#v", err, extra)
	}
}

func TestGremlins_RejectsOutputEscapingRoot(t *testing.T) {
	t.Parallel()
	mem := NewMemoryFileOps()
	response := Gremlins(mem, "/root", []byte(`{}`))
	cmd := cmdrunner.NewCommand(".", "go", "run", "gremlins", "unleash", "-o", "../mutation.json")
	err := response(context.Background(), cmd, io.Discard, io.Discard)
	if !errors.Is(err, ErrGremlinsReportPathEscape) {
		t.Fatalf("Gremlins error = %v, want ErrGremlinsReportPathEscape", err)
	}
}

func TestGremlins_WritesReport(t *testing.T) {
	t.Parallel()
	mem := NewMemoryFileOps()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	report := []byte(`{"files":[]}`)
	const root = "/root"
	const outPath = "artifacts/mutation.json"
	response := Gremlins(mem, root, report)
	cmd := cmdrunner.NewCommand(".", "go", "run",
		"github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.0",
		"unleash", "-o", outPath,
	)
	if err := response(context.Background(), cmd, io.Discard, io.Discard); err != nil {
		t.Fatalf("Gremlins error: %v", err)
	}
	wantPath, err := gremlinsReportLogicalDestination(root, outPath)
	if err != nil {
		t.Fatalf("gremlinsReportLogicalDestination: %v", err)
	}
	data, readErr := mem.ReadFile(wantPath)
	if readErr != nil {
		t.Fatalf("mutation report not written at %q: %v", wantPath, readErr)
	}
	if string(data) != `{"files":[]}` {
		t.Fatalf("report content = %q", string(data))
	}
}

func TestGremlins_AbsoluteOutputUnderRootWritesLogicalRelativeReport(t *testing.T) {
	t.Parallel()
	mem := NewMemoryFileOps()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	report := []byte(`{"files":[]}`)
	const root = "/root"
	const outPath = "/root/artifacts/mutation.json"
	response := Gremlins(mem, root, report)
	cmd := cmdrunner.NewCommand(root, "go", "run",
		"github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.0",
		"unleash", "-o", outPath,
	)
	if err := response(context.Background(), cmd, io.Discard, io.Discard); err != nil {
		t.Fatalf("Gremlins error: %v", err)
	}
	if _, err := mem.ReadFile("artifacts/mutation.json"); err != nil {
		t.Fatalf("mutation report not written at logical relative path: %v", err)
	}
}

func TestGremlins_RelativeOutputIgnoresCommandDir(t *testing.T) {
	t.Parallel()
	mem := NewMemoryFileOps()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	report := []byte(`{"files":[]}`)
	response := Gremlins(mem, "", report)

	first := cmdrunner.NewCommand("/first", "go", "run", "gremlins", "unleash", "-o", "mutation.json")
	if err := response(context.Background(), first, io.Discard, io.Discard); err != nil {
		t.Fatalf("first Gremlins call: %v", err)
	}
	second := cmdrunner.NewCommand("/second", "go", "run", "gremlins", "unleash", "-o", "mutation.json")
	if err := response(context.Background(), second, io.Discard, io.Discard); err != nil {
		t.Fatalf("second Gremlins call: %v", err)
	}

	if _, err := mem.ReadFile("mutation.json"); err != nil {
		t.Fatalf("relative report not written at command argv path: %v", err)
	}
}

func TestModulePathQuery_UnsupportedFormatErrors(t *testing.T) {
	t.Parallel()
	h := GoList("example.com/mod", "/mod", map[string]PackageListInfo{
		"example.com/mod/pkg": DirOnly("/mod/pkg"),
	})
	cmd := cmdrunner.NewCommand(".", "go", "list", "-m", "-f", "{{.Unknown}}")
	var stdout strings.Builder
	err := h(context.Background(), cmd, &stdout, io.Discard)
	if !errors.Is(err, errGoListFakeUnsupportedFormat) {
		t.Fatalf("GoList unsupported format error = %v", err)
	}
}
