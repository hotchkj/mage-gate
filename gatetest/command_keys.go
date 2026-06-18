// Vision: Canonical FakeRunner command keys for BDD—one source of truth so steps and tests do not drift literals.
package gatetest

// LocalToolState represents the resolution state of a locally installed tool binary.
type LocalToolState string

const (
	ToolStateMatching    LocalToolState = "matching"
	ToolStateMismatched  LocalToolState = "mismatched"
	ToolStateMissing     LocalToolState = "missing"
	ToolStateUnprobeable LocalToolState = "unprobeable"
	ToolStateDefault     LocalToolState = ""
)

func VetCommandKeys() []string      { return []string{"go vet"} }
func CompileCommandKeys() []string  { return []string{"go build"} }
func TestCommandKeys() []string     { return []string{"go test"} }
func DurationCommandKeys() []string { return nil }

// ResolverExpectedKeys returns the expected command keys for a tool that uses ToolResolver.
// toolName is the local binary name (e.g. "golangci-lint", "deadcode", "gremlins", "gocyclo").
// spec is the full module spec (e.g. "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1").
// state determines whether the tool resolves locally or via go run.
func ResolverExpectedKeys(toolName, spec string, state LocalToolState) []string {
	switch state {
	case ToolStateMatching:
		return []string{toolName}
	case ToolStateMismatched, ToolStateMissing:
		if spec != "" {
			return []string{"go run " + spec}
		}
		return []string{"go run"}
	case ToolStateUnprobeable:
		return nil
	case ToolStateDefault:
		return []string{"go run"}
	default:
		return []string{"go run"}
	}
}
