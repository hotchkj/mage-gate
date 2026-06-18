// Vision: Duration step: read `go test -json` events, enforce per-test max seconds, and ignore package wall-clock.
package harness

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// StepDuration reads test-events.jsonl from the artifact store (from the upstream go test step) and checks durations.
func (h *StepRunner) StepDuration(
	_ context.Context,
	unitMaxSeconds float64,
	upstreamStepID string,
) error {
	if err := h.requireDurationStepConfig(unitMaxSeconds, upstreamStepID); err != nil {
		return err
	}
	tests, err := h.loadTestDurations(upstreamStepID)
	if err != nil {
		return err
	}
	return h.runDurationCheck(tests, unitMaxSeconds)
}

func (h *StepRunner) requireDurationStepConfig(unitMaxSeconds float64, upstreamStepID string) error {
	if unitMaxSeconds <= 0 {
		return fmt.Errorf("%w: unit_max_seconds must be > 0 for duration step", ErrDurationFailed)
	}
	if upstreamStepID == "" {
		return fmt.Errorf("%w: upstream step ID not configured", ErrDurationFailed)
	}
	return nil
}

func (h *StepRunner) loadTestDurations(upstreamStepID string) ([]gatecheck.TestDuration, error) {
	eventsData, err := h.store.Read(upstreamStepID, "test-events.jsonl")
	if err != nil {
		return nil, fmt.Errorf("%w: read test events from store: %w", ErrDurationFailed, err)
	}
	tests, err := gatecheck.ParseTestEvents(bytes.NewReader(eventsData))
	if err != nil {
		return nil, fmt.Errorf("%w: parse test events: %w", ErrDurationFailed, err)
	}
	return tests, nil
}

func (h *StepRunner) runDurationCheck(tests []gatecheck.TestDuration, unitMaxSeconds float64) error {
	result, err := gatecheck.DurationFromTests(tests, unitMaxSeconds)
	if err != nil {
		return fmt.Errorf("%w: duration check: %w", ErrDurationFailed, err)
	}
	if !result.Passed {
		return fmt.Errorf("%w: %s", ErrDurationFailed, gatecheck.FormatDurationReport(result))
	}
	return nil
}
