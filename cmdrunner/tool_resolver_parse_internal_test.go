// Vision: White-box tests for go list / go version -m parsing helpers—malformed output surfaces sentinel errors.
package cmdrunner

import (
	"errors"
	"testing"
)

func TestParseModuleFromVersionOutput_BinaryModuleMetadataAbsent_NoModLine(t *testing.T) {
	t.Parallel()
	output := "path/to/binary binary\n\tstd v1.20"
	_, _, err := parseModuleFromVersionOutput(output)
	if err == nil {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got nil")
	}
	if !errors.Is(err, errBinaryModuleMetadataAbsent) {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got %v", err)
	}
}

func TestParseModuleFromVersionOutput_BinaryModuleMetadataAbsent_EmptyModPath(t *testing.T) {
	t.Parallel()
	output := "path/to/binary binary\n\tmod  v1.0.0"
	_, _, err := parseModuleFromVersionOutput(output)
	if err == nil {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got nil")
	}
	if !errors.Is(err, errBinaryModuleMetadataAbsent) {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got %v", err)
	}
}

func TestParseModuleFromVersionOutput_BinaryModuleMetadataAbsent_EmptyModVersion(t *testing.T) {
	t.Parallel()
	output := "path/to/binary binary\n\tmod github.com/example/pkg "
	_, _, err := parseModuleFromVersionOutput(output)
	if err == nil {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got nil")
	}
	if !errors.Is(err, errBinaryModuleMetadataAbsent) {
		t.Fatalf("expected errBinaryModuleMetadataAbsent, got %v", err)
	}
}

func TestParseGoListMOutput_EmptyLine_ReturnsErrGoListEmptyOutput(t *testing.T) {
	t.Parallel()
	const spec = "github.com/example/pkg@v1.0.0"
	_, _, err := parseGoListMOutput("", spec)
	if err == nil {
		t.Fatalf("expected errGoListEmptyOutput, got nil")
	}
	if !errors.Is(err, errGoListEmptyOutput) {
		t.Fatalf("expected errGoListEmptyOutput, got %v", err)
	}
}

func TestParseGoListMOutput_UnexpectedFormat_ReturnsErrGoListUnexpectedFormat(t *testing.T) {
	t.Parallel()
	const spec = "github.com/example/pkg@v1.0.0"
	_, _, err := parseGoListMOutput("onlyonefield", spec)
	if err == nil {
		t.Fatalf("expected errGoListUnexpectedFormat, got nil")
	}
	if !errors.Is(err, errGoListUnexpectedFormat) {
		t.Fatalf("expected errGoListUnexpectedFormat, got %v", err)
	}
}

func TestParseModuleFromVersionOutput_Success(t *testing.T) {
	t.Parallel()
	output := "/path/to/binary binary\n\tmod github.com/example/pkg v1.0.0\n"
	modPath, modVersion, err := parseModuleFromVersionOutput(output)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if modPath != "github.com/example/pkg" {
		t.Errorf("modPath = %q, want %q", modPath, "github.com/example/pkg")
	}
	if modVersion != "v1.0.0" {
		t.Errorf("modVersion = %q, want %q", modVersion, "v1.0.0")
	}
}

func TestParseGoListM_Success(t *testing.T) {
	t.Parallel()
	const spec = "github.com/example/pkg@v1.0.0"
	line := "github.com/example/pkg v1.0.0"
	modPath, modVersion, err := parseGoListMOutput(line, spec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if modPath != "github.com/example/pkg" {
		t.Errorf("modPath = %q, want %q", modPath, "github.com/example/pkg")
	}
	if modVersion != "v1.0.0" {
		t.Errorf("modVersion = %q, want %q", modVersion, "v1.0.0")
	}
}
