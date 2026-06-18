package testfail

import "testing"

func TestNoise_PASS_EVENT_SPAM(t *testing.T) {
	t.Log("PASS_EVENT_SPAM: this line should be filtered in agent diagnostics")
}

func TestAlwaysFails(t *testing.T) {
	if Add(1, 1) != 3 {
		t.Fatal("intentional failing test fixture")
	}
}
