// This fixture intentionally fails in a fixed delay to drive duration-fault diagnostics
// in integration tests.
// Known-bad: expected failure path; keep as a controlled delay fixture.
package durationfail

import "testing"
import "time"

func TestSlow(t *testing.T) { time.Sleep(2 * time.Second) }
