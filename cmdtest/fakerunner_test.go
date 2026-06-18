// Vision: FakeRunner dispatch table, call recording, and strict failure when no handler matches a command key.
package cmdtest_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdrunner"
	"github.com/hotchkj/mage-gate/cmdtest"
)

var errFakeResponse = errors.New("fake response error")

func TestFakeRunner_ExactKeyDispatch(t *testing.T) {
	t.Parallel()
	var called bool
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			called = true
			return nil
		}),
	)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "test", "./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("response was not called")
	}
}

func TestFakeRunner_TwoLevelKey(t *testing.T) {
	t.Parallel()
	var matched string
	responseForKey := func(key string) cmdtest.CommandFunc {
		return func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			matched = key
			return nil
		}
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", responseForKey("go test")),
		cmdtest.On("go vet", responseForKey("go vet")),
	)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched != "go test" {
		t.Fatalf("matched %q, want go test", matched)
	}
}

func TestFakeRunner_ThreeLevelKey(t *testing.T) {
	t.Parallel()
	var matched string
	responseForKey := func(key string) cmdtest.CommandFunc {
		return func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			matched = key
			return nil
		}
	}
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go tool cover", responseForKey("go tool cover")),
		cmdtest.On("go tool", responseForKey("go tool")),
	)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "tool", "cover", "-func=cov.out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched != "go tool cover" {
		t.Fatalf("matched %q, want go tool cover", matched)
	}
}

func TestFakeRunner_Fallback(t *testing.T) {
	t.Parallel()
	var matched string
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("mytool", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			matched = "mytool"
			return nil
		}),
	)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "mytool", "subcmd", "arg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched != "mytool" {
		t.Fatalf("matched %q, want mytool", matched)
	}
}

func TestFakeRunner_UnhandledReturnsError(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner()
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "unknown", "cmd")
	if err == nil {
		t.Fatal("expected error for unhandled command")
	}
	if !errors.Is(err, cmdtest.ErrUnhandledCommand) {
		t.Fatalf("expected ErrUnhandledCommand, got: %v", err)
	}
}

func TestFakeRunner_CallRecordedBeforeResponse(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return errFakeResponse
		}),
	)
	err := runner.Run(context.Background(), "/project", io.Discard, io.Discard, "go", "test", "./...")
	if !errors.Is(err, errFakeResponse) {
		t.Fatalf("expected errFakeResponse, got %v", err)
	}
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name() != "go" {
		t.Fatalf("call name = %q", calls[0].Name())
	}
	if calls[0].Arg(0) != "test" {
		t.Fatalf("call arg0 = %q", calls[0].Arg(0))
	}
}

func TestFakeRunner_CallsReturnsCopy(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return nil
		}),
	)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls1 := runner.Calls()
	calls2 := runner.Calls()
	if len(calls1) != len(calls2) {
		t.Fatal("calls slices differ in length")
	}
	calls1[0] = cmdrunner.NewCommand(".", "mutated")
	fresh := runner.Calls()
	if fresh[0].Name() == "mutated" {
		t.Fatal("mutating Calls() result affected internal state")
	}
	if len(fresh) != 1 {
		t.Fatal("expected 1 call after mutation")
	}
}

func TestFakeRunner_DuplicateOnPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic for duplicate On()")
		}
	}()
	noop := func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error { return nil }
	first := cmdtest.On("go test", noop)
	second := cmdtest.On("go test", noop)
	cmdtest.NewFakeRunner(first, second)
}

func TestFakeRunner_ResponseReceivesStdoutStderr(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go list", func(_ context.Context, _ cmdrunner.Command, stdout, stderr io.Writer) error {
			if _, err := io.WriteString(stdout, "pkg/a\n"); err != nil {
				return err
			}
			_, err := io.WriteString(stderr, "warning\n")
			return err
		}),
	)
	var stdout, stderr strings.Builder
	err := runner.Run(context.Background(), ".", &stdout, &stderr, "go", "list", "./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "pkg/a\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "warning\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestFakeRunner_DirCanonicalized(t *testing.T) {
	t.Parallel()
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
			return nil
		}),
	)
	err := runner.Run(context.Background(), "", io.Discard, io.Discard, "go", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := runner.Calls()
	if calls[0].Dir() != "." {
		t.Fatalf("Dir() = %q, want \".\"", calls[0].Dir())
	}
}

func TestFakeRunner_MultipleCallsRecorded(t *testing.T) {
	t.Parallel()
	noop := func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error { return nil }
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("go test", noop),
		cmdtest.On("go list", noop),
	)
	err1 := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "test")
	if err1 != nil {
		t.Fatalf("first Run error: %v", err1)
	}
	err2 := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "list", "./...")
	if err2 != nil {
		t.Fatalf("second Run error: %v", err2)
	}
	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Arg(0) != "test" {
		t.Fatalf("first call arg0 = %q", calls[0].Arg(0))
	}
	if calls[1].Arg(0) != "list" {
		t.Fatalf("second call arg0 = %q", calls[1].Arg(0))
	}
}

func TestValidateUniqueResponseKeys_AcceptsDistinctKeys(t *testing.T) {
	t.Parallel()
	noop := func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error { return nil }
	err := cmdtest.ValidateUniqueResponseKeys(
		cmdtest.On("go test", noop),
		cmdtest.On("go list", noop),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateUniqueResponseKeys_DuplicateKeyReturnsError(t *testing.T) {
	t.Parallel()
	noop := func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error { return nil }
	opts := make([]cmdtest.RunnerOption, 2)
	for i := range opts {
		opts[i] = cmdtest.On("go test", noop)
	}
	err := cmdtest.ValidateUniqueResponseKeys(opts...)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, cmdtest.ErrDuplicateResponseKey) {
		t.Fatalf("errors.Is duplicate: got %v", err)
	}
}

func TestValidateUniqueResponseKeys_RejectsMergeOption(t *testing.T) {
	t.Parallel()
	noop := func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error { return nil }
	err := cmdtest.ValidateUniqueResponseKeys(
		cmdtest.MergeDuplicateKeys(),
		cmdtest.On("go test", noop),
	)
	if !errors.Is(err, cmdtest.ErrValidateUniqueContainsMerge) {
		t.Fatalf("got %v", err)
	}
}

func TestFakeRunnerMergeDuplicateKeys_FirstResponseWins(t *testing.T) {
	t.Parallel()
	var called string
	first := cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
		called = "first"
		return nil
	})
	second := cmdtest.On("go test", func(_ context.Context, _ cmdrunner.Command, _, _ io.Writer) error {
		called = "second"
		return nil
	})
	runner := cmdtest.NewFakeRunner(cmdtest.MergeDuplicateKeys(), first, second)
	err := runner.Run(context.Background(), ".", io.Discard, io.Discard, "go", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != "first" {
		t.Fatalf("called = %q, want first (first response should win)", called)
	}
}

func TestFakeRunner_CommandDirMatchesDirNativeForExec(t *testing.T) {
	t.Parallel()
	const dir = "a/./b/../c"
	runner := cmdtest.NewFakeRunner(
		cmdtest.On("noop", func(_ context.Context, cmd cmdrunner.Command, _, _ io.Writer) error {
			want := cmdrunner.DirNativeForExec(dir)
			if cmd.Dir() != want {
				t.Fatalf("recorded cwd %q differs from DirNativeForExec(%q)==%q", cmd.Dir(), dir, want)
			}
			return nil
		}),
	)
	if err := runner.Run(context.Background(), dir, io.Discard, io.Discard, "noop"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
