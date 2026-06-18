// Vision: Configurable ToolResolver double—pin local paths or force go-run without executing real installs.
package gatetest

import (
	"context"
	"errors"
	"fmt"
)

var errToolProbeFailed = errors.New("probe tool version failed")

// FakeToolResolver implements cmdrunner.ToolResolver for testing.
// It records calls and can be configured with per-tool responses.
type FakeToolResolver struct {
	// localMatches maps "toolName@spec" to whether it should match the local binary.
	// Example: "golangci-lint@github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1" -> true
	localMatches   map[string]bool
	defaultToLocal bool
	calls          []ToolResolutionCall
}

// ToolResolutionCall records a call to ResolveToolCommand.
type ToolResolutionCall struct {
	ToolName  string
	ToolSpec  string
	ExtraArgs []string
	Result    string // "local" or "gorun" or "error"
	ErrorMsg  string
}

// NewFakeToolResolver creates a FakeToolResolver with no local matches by default.
// All resolution requests will fall back to "go run".
func NewFakeToolResolver() *FakeToolResolver {
	return &FakeToolResolver{
		localMatches: make(map[string]bool),
		calls:        []ToolResolutionCall{},
	}
}

// SetLocalMatch configures whether a tool matches a specific spec.
// Example: SetLocalMatch("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)
func (f *FakeToolResolver) SetLocalMatch(toolName, spec string, matches bool) {
	key := toolName + "@" + spec
	f.localMatches[key] = matches
}

// SetLocalMatchFull is like SetLocalMatch but uses the full spec key.
// Example: SetLocalMatchFull("golangci-lint@github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1", true)
func (f *FakeToolResolver) SetLocalMatchFull(key string, matches bool) {
	f.localMatches[key] = matches
}

// SetDefaultToLocal configures the resolver to return local tools by default
// for any tool@spec pair, unless explicitly overridden via SetLocalMatch.
// This is useful for tests that just want "return the tool name directly".
func (f *FakeToolResolver) SetDefaultToLocal(defaultToLocal bool) *FakeToolResolver {
	f.defaultToLocal = defaultToLocal
	return f
}

// ResolveToolCommand implements cmdrunner.ToolResolver.
// If SetLocalMatch configured the tool@spec as matching, returns the local tool path.
// If SetDefaultToLocal(true) is set, returns the local tool by default (unless explicitly set to false).
// Otherwise returns "go run <spec>" fallback.
func (f *FakeToolResolver) ResolveToolCommand(
	ctx context.Context,
	toolName, toolSpec string,
	extraArgs []string,
) (binary string, args []string, err error) {
	// Build the key for this tool@spec pair.
	key := toolName + "@" + toolSpec
	if matches, ok := f.localMatches[key]; ok && matches {
		// Local binary matches; use it.
		record := ToolResolutionCall{
			ToolName:  toolName,
			ToolSpec:  toolSpec,
			ExtraArgs: append([]string{}, extraArgs...),
			Result:    "local",
		}
		f.calls = append(f.calls, record)
		return toolName, extraArgs, nil
	}

	// Check if we should default to local.
	if f.defaultToLocal {
		if matches, ok := f.localMatches[key]; !ok || matches {
			// Either not explicitly set to false, or explicitly set to true.
			record := ToolResolutionCall{
				ToolName:  toolName,
				ToolSpec:  toolSpec,
				ExtraArgs: append([]string{}, extraArgs...),
				Result:    "local",
			}
			f.calls = append(f.calls, record)
			return toolName, extraArgs, nil
		}
	}

	// Fall back to go run.
	args = []string{"run", toolSpec}
	args = append(args, extraArgs...)
	record := ToolResolutionCall{
		ToolName:  toolName,
		ToolSpec:  toolSpec,
		ExtraArgs: append([]string{}, extraArgs...),
		Result:    "gorun",
	}
	f.calls = append(f.calls, record)
	return "go", args, nil
}

// Calls returns the recorded tool resolution calls.
func (f *FakeToolResolver) Calls() []ToolResolutionCall {
	return f.calls
}

// FakeToolResolverError is a FakeToolResolver that always returns an error for testing error handling.
type FakeToolResolverError struct {
	errorMsg string
}

// NewFakeToolResolverError creates a resolver that always returns an error.
func NewFakeToolResolverError(msg string) *FakeToolResolverError {
	return &FakeToolResolverError{errorMsg: msg}
}

// ResolveToolCommand always returns an error.
func (f *FakeToolResolverError) ResolveToolCommand(
	ctx context.Context,
	toolName, toolSpec string,
	extraArgs []string,
) (binary string, args []string, err error) {
	return "", nil, fmt.Errorf("%w: %s: %s", errToolProbeFailed, toolName, f.errorMsg)
}
