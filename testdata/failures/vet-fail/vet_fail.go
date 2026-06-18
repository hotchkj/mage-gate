// Package vetfail provides code that go vet rejects.
// This fixture is used to verify vet step failure detection.
package vetfail

import "fmt"

// VetFail has a format string mismatch that go vet catches.
func VetFail() {
	fmt.Printf("%d", "string") //nolint:govet // intentional vet failure for fixture
}
