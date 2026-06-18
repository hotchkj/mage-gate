// Vision: Regex/line parsers used by BDD assertions on gatecheck text output (stable anchors for features/).
package gatecheck

import (
	"regexp"
	"testing"
)

func TestCoverFuncRegexMatchesBDDSyntheticLine(t *testing.T) {
	t.Parallel()
	line := "example.com/mod/file.go:10:\tValidate\t\t95.0%"
	re := regexp.MustCompile(`^(.+):(\d+):\s+(\S+)\s+([\d.]+)%$`)
	if re.FindStringSubmatch(line) == nil {
		t.Fatalf("regex should match BDD fake cover line: %q", line)
	}
}

func TestCrap_BDDSyntheticInputs(t *testing.T) {
	t.Parallel()
	gocyclo := "5 pkg Validate file.go:1:1\n"
	cover := "example.com/mod/file.go:10:\tValidate\t\t95.0%\ntotal:\t(statements)\t95.0%\n"
	result, err := Crap(gocyclo, cover, "example.com/mod", "/mod", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass, got offenders %+v", result.Offenders)
	}
}

func TestCrap_ShortCoverPathEmptyModule(t *testing.T) {
	t.Parallel()
	gocyclo := "5 pkg Validate file.go:1:1\n"
	cover := "file.go:10:\tValidate\t\t95.0%\ntotal:\t(statements)\t95.0%\n"
	result, err := Crap(gocyclo, cover, "", "/mod", 8.0, nil)
	if err != nil {
		t.Fatalf("Crap: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass with empty module, got offenders %+v", result.Offenders)
	}
}
