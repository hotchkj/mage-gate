//go:build qgdcroots_f4c8e91b3d7042a6b5c1e8f9037d2a61

// Vision: Compile-only RTA anchors for golang.org/x/tools/cmd/deadcode when the step
// passes -tags matching DeadcodeRootsBuildTag. Default builds omit this file; init does
// not run in consumer workflows.
//
// Only symbols that are (a) legitimately exported, (b) forbidden from ordinary tests by
// forbidigo because they wire real OS/subprocess behavior, and (c) therefore invisible
// to the test RTA graph belong here. Wrappers such as NewDisplayRunner are not listed:
// tests call them with injected fakes and deadcode already sees them as reachable.
package gate

func init() {
	// Production-only constructors: forbidden in tests by forbidigo rules.
	// These are the only gate symbols unreachable from the test call graph by design.
	var fo FileOps
	fo = NewProductionFileOps()
	_ = fo

	var cmdRunner CommandRunner
	cmdRunner = NewProductionRunner()
	_ = cmdRunner

	var toolResolver ToolResolver
	toolResolver = NewProductionToolResolver()
	_ = toolResolver
}
