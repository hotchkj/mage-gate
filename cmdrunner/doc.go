// Package cmdrunner defines command invocation records, the CommandRunner interface, production
// runners, and related subprocess tooling. It is gate-agnostic: it does not import gate, harness,
// or mage targets. Production code depends on the standard library plus
// github.com/hotchkj/mage-gate/internal/fsnorm for path normalization only; extracting
// cmdrunner to a standalone module requires addressing that dependency (e.g. copy fsnorm or
// replace with a small path helper).
//
// Consumers that need deterministic tool-resolution behavior in their own tests should inject
// a ToolResolver fake at their package boundary. The concrete resolver returned by
// NewProductionToolResolver is production wiring; its local-vs-go-run probing is tested by
// cmdrunner itself.
package cmdrunner
