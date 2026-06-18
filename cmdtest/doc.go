// Package cmdtest provides test doubles for [github.com/hotchkj/mage-gate/cmdrunner]:
// keyed fake runners and canned responses. Use it to test code that builds and runs commands:
// inspect argv/cwd, simulate stdout/stderr, and exercise failure handling without spawning
// subprocesses. Code that depends on ToolResolver should inject its own fake resolver rather
// than retesting cmdrunner's concrete resolver implementation.
//
// It depends only on cmdrunner (and the standard library); it does not import gate, harness,
// or mage targets, and is suitable to extract as a separate module that depends on cmdrunner
// alone.
package cmdtest
