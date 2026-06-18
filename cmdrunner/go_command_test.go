// Vision: GoCommand flag matrix: supported go test / toolchain flag spellings the harness must recognize.
package cmdrunner_test

import (
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
)

func TestGoCommand_HasFlag(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-json", "-coverprofile=cov.out", "./...")
	goCmd := cmd.Go()
	if !goCmd.HasFlag("json") {
		t.Fatal("HasFlag(json) should be true")
	}
	if !goCmd.HasFlag("coverprofile") {
		t.Fatal("HasFlag(coverprofile) should be true")
	}
	if goCmd.HasFlag("race") {
		t.Fatal("HasFlag(race) should be false")
	}
}

func TestGoCommand_HasFlag_StopsAtDoubleDash(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-json", "--", "-hidden")
	goCmd := cmd.Go()
	if !goCmd.HasFlag("json") {
		t.Fatal("HasFlag(json) should be true before --")
	}
	if goCmd.HasFlag("hidden") {
		t.Fatal("HasFlag(hidden) should be false after --")
	}
}

func TestGoCommand_FlagValue_EqualsForm(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile=coverage.out", "./...")
	goCmd := cmd.Go()
	val, ok := goCmd.FlagValue("coverprofile")
	if !ok {
		t.Fatal("FlagValue should find coverprofile")
	}
	if val != "coverage.out" {
		t.Fatalf("FlagValue = %q, want coverage.out", val)
	}
}

func TestGoCommand_FlagValue_TwoTokenForm(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile", "coverage.out", "./...")
	goCmd := cmd.Go()
	val, ok := goCmd.FlagValue("coverprofile")
	if !ok {
		t.Fatal("FlagValue should find coverprofile (two-token)")
	}
	if val != "coverage.out" {
		t.Fatalf("FlagValue = %q, want coverage.out", val)
	}
}

func TestGoCommand_FlagValue_BooleanFlagAtEnd(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...", "-json")
	goCmd := cmd.Go()
	val, ok := goCmd.FlagValue("json")
	if !ok {
		t.Fatal("FlagValue should find json")
	}
	if val != "" {
		t.Fatalf("boolean flag at end value = %q, want empty", val)
	}
}

func TestGoCommand_HasFlag_Boolean(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-json", "./...")
	if !cmd.Go().HasFlag("json") {
		t.Fatal("HasFlag(json) should be true")
	}
}

func TestGoCommand_FlagValue_NotFound(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...")
	goCmd := cmd.Go()
	_, ok := goCmd.FlagValue("race")
	if ok {
		t.Fatal("FlagValue(race) should return false")
	}
}

func TestGoCommand_FlagValue_AfterDoubleDash(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "--", "-coverprofile=cov.out")
	goCmd := cmd.Go()
	_, ok := goCmd.FlagValue("coverprofile")
	if ok {
		t.Fatal("FlagValue should not find flag after --")
	}
}

func TestGoCommand_Flags_AllForms(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test",
		"-json",
		"-coverprofile=cov.out",
		"-count=1",
		"./...",
	)
	goCmd := cmd.Go()
	flags := goCmd.Flags()
	assertFlagValues(t, flags, "json", []string{""})
	assertFlagValues(t, flags, "coverprofile", []string{"cov.out"})
	assertFlagValues(t, flags, "count", []string{"1"})
}

func TestGoCommand_FlagValue_TwoTokenNotInFlags(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-coverprofile", "cov.out")
	goCmd := cmd.Go()
	val, ok := goCmd.FlagValue("coverprofile")
	if !ok || val != "cov.out" {
		t.Fatalf("FlagValue should find two-token form, got %q ok=%v", val, ok)
	}
	flags := goCmd.Flags()
	vals := flags["coverprofile"]
	if len(vals) != 1 || vals[0] != "" {
		t.Fatalf("Flags() should see bare -coverprofile (no value), got %v", vals)
	}
}

func TestGoCommand_Flags_RepeatedPreserved(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test",
		"-run=TestA",
		"-run=TestB",
	)
	goCmd := cmd.Go()
	flags := goCmd.Flags()
	assertFlagValues(t, flags, "run", []string{"TestA", "TestB"})
}

func TestGoCommand_Flags_StopsAtDoubleDash(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "-json", "--", "-hidden=val")
	goCmd := cmd.Go()
	flags := goCmd.Flags()
	if _, ok := flags["hidden"]; ok {
		t.Fatal("flags after -- should be excluded")
	}
	assertFlagValues(t, flags, "json", []string{""})
}

func TestGoCommand_Flags_NoFlags(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go", "test", "./...")
	goCmd := cmd.Go()
	flags := goCmd.Flags()
	if len(flags) != 0 {
		t.Fatalf("expected empty flags map, got %v", flags)
	}
}

func TestGoCommand_Flags_EmptyArgs(t *testing.T) {
	t.Parallel()
	cmd := cmdrunner.NewCommand(".", "go")
	goCmd := cmd.Go()
	flags := goCmd.Flags()
	if len(flags) != 0 {
		t.Fatalf("expected empty flags map, got %v", flags)
	}
}

func assertFlagValues(
	t *testing.T,
	flags map[string][]string,
	key string,
	want []string,
) {
	t.Helper()
	got, ok := flags[key]
	if !ok {
		t.Fatalf("flags[%q] not found", key)
	}
	if len(got) != len(want) {
		t.Fatalf("flags[%q] len = %d, want %d: %v", key, len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] { // #nosec G602 -- len guard above guarantees want[i] is in range
			t.Fatalf("flags[%q][%d] = %q, want %q", key, i, v, want[i]) // #nosec G602
		}
	}
}
