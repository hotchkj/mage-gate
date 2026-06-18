// Vision: BDD toolchain fakes keep coverage profiles and go tool cover output aligned with scenario intent.
package steps

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
	"golang.org/x/tools/cover"
)

func TestGoTestOptsDefaultUsesMinimalCoverProfileFullStatementCoverage(t *testing.T) {
	t.Parallel()

	s := newScenarioState()
	mem := s.ensureMem()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	fr := cmdtest.NewFakeRunner(s.goTestOpts()...)
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "test", "-coverprofile=artifacts/coverage.out", "./...",
	); err != nil {
		t.Fatalf("fake go test: %v", err)
	}
	data, err := mem.ReadFile("artifacts/coverage.out")
	if err != nil {
		t.Fatalf("read coverprofile: %v", err)
	}
	if got := statementCoveragePercent(t, string(data)); got != 100.0 {
		t.Fatalf("default fake profile statement coverage = %g, want 100", got)
	}
}

func TestGoTestOptsExplicitZeroWritesZeroCoverageProfile(t *testing.T) {
	t.Parallel()

	scenario := newScenarioState()
	if err := scenario.givenCodebaseTestCoverage(0); err != nil {
		t.Fatalf("givenCodebaseTestCoverage: %v", err)
	}
	mem := scenario.ensureMem()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	fr := cmdtest.NewFakeRunner(scenario.goTestOpts()...)
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "test", "-coverprofile=artifacts/coverage.out", "./...",
	); err != nil {
		t.Fatalf("fake go test: %v", err)
	}
	data, err := mem.ReadFile("artifacts/coverage.out")
	if err != nil {
		t.Fatalf("read coverprofile: %v", err)
	}
	if got := statementCoveragePercent(t, string(data)); got != 0 {
		t.Fatalf("explicit 0%% fake profile statement coverage = %g, want 0", got)
	}
}

func TestGoToolCoverOptsDefaultMatchesMinimalProfilePercent(t *testing.T) {
	t.Parallel()

	s := newScenarioState()
	fr := cmdtest.NewFakeRunner(s.goToolCoverOpts()...)
	want := "total:\t(statements)\t100.0%\n"
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "tool", "cover", "-func=x",
	); err != nil {
		t.Fatalf("fake go tool cover: %v", err)
	}
	if got := stdout.String(); got != want {
		t.Fatalf("go tool cover stdout = %q, want %q", got, want)
	}
}

func TestGoToolCoverOptsExplicitZeroReportsZeroPercent(t *testing.T) {
	t.Parallel()

	s := newScenarioState()
	if err := s.givenCodebaseTestCoverage(0); err != nil {
		t.Fatalf("givenCodebaseTestCoverage: %v", err)
	}
	fr := cmdtest.NewFakeRunner(s.goToolCoverOpts()...)
	want := "total:\t(statements)\t0.0%\n"
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "tool", "cover", "-func=x",
	); err != nil {
		t.Fatalf("fake go tool cover: %v", err)
	}
	if got := stdout.String(); got != want {
		t.Fatalf("go tool cover stdout = %q, want %q", got, want)
	}
}

func TestGoTestOptsCoverprofileTwoTokenFlagMatchesEqualityForm(t *testing.T) {
	t.Parallel()

	s := newScenarioState()
	mem := s.ensureMem()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	fr := cmdtest.NewFakeRunner(s.goTestOpts()...)
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "test", "-coverprofile", "artifacts/coverage.out", "./...",
	); err != nil {
		t.Fatalf("fake go test (two-token -coverprofile): %v", err)
	}
	if _, err := mem.ReadFile("artifacts/coverage.out"); err != nil {
		t.Fatalf("read coverprofile: %v", err)
	}
}

func TestSlowTestResponseWritesCoverprofileTwoToken(t *testing.T) {
	t.Parallel()

	scenario := newScenarioState()
	if err := scenario.givenSlowTests(); err != nil {
		t.Fatalf("givenSlowTests: %v", err)
	}
	mem := scenario.ensureMem()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	fr := cmdtest.NewFakeRunner(scenario.composeDurationOpts()...)
	var stdout strings.Builder
	if err := fr.Run(
		context.Background(), ".", &stdout, io.Discard,
		"go", "test", "-json", "-coverprofile", "artifacts/duration.cover", "./...",
	); err != nil {
		t.Fatalf("fake duration go test: %v", err)
	}
	if _, err := mem.ReadFile("artifacts/duration.cover"); err != nil {
		t.Fatalf("slow-test coverprofile: %v", err)
	}
}

func TestExplicitInvalidCoverageRejectedByGoTestFake(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pct  int
	}{
		{name: "negative", pct: -1},
		{name: "over100", pct: 101},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newScenarioState()
			s.testCovPercent = tc.pct
			s.testCovExplicit = true
			mem := s.ensureMem()
			fr := cmdtest.NewFakeRunner(s.goTestOpts()...)
			var stdout strings.Builder
			err := fr.Run(
				context.Background(), ".", &stdout, io.Discard,
				"go", "test", "-coverprofile=out", "./...",
			)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, gatetest.ErrFakeCoveragePercentOutOfRange) {
				t.Fatalf("errors.Is out of range sentinel: got %v", err)
			}
			if _, readErr := mem.ReadFile("out"); readErr == nil {
				t.Fatal("unexpected coverprofile written for invalid percent")
			}
		})
	}
}

func statementCoveragePercent(t *testing.T, profile string) float64 {
	t.Helper()

	profiles, err := cover.ParseProfilesFromReader(strings.NewReader(profile))
	if err != nil {
		t.Fatalf("cover.ParseProfilesFromReader: %v", err)
	}
	var covered, total int
	for _, p := range profiles {
		for _, b := range p.Blocks {
			total += b.NumStmt
			if b.Count > 0 {
				covered += b.NumStmt
			}
		}
	}
	if total == 0 {
		t.Fatal("profile has no statements")
	}
	return 100.0 * float64(covered) / float64(total)
}
