// Vision: Opaque step output tokens and monotonic step IDs for artifact correlation.
package gate

import (
	"fmt"
	"sync/atomic"

	"github.com/hotchkj/mage-gate/internal/gatecheck"
)

// counter is process-wide monotonic IDs—unique enough within one [ArtifactStore], not stable across processes.
var counter atomic.Uint64

// TestOutput is a token proving a Test step produced artifacts.
// Zero value is invalid; obtain only from [Test] or [CoveredTestOutput.TestRun].
type TestOutput struct {
	stepID    string
	scope     PackageScope
	qualifier string
}

// CoveredTestOutput is a token proving a CoveredTest step produced coverage artifacts.
// Zero value is invalid; obtain only from [CoveredTest].
type CoveredTestOutput struct {
	stepID       string
	packages     PackageScope
	qualityScope QualityScope
	qualifier    string
}

// TestRun builds [Duration] input (same stepID and [PackageScope] as [CoveredTest]).
// Invalid receiver or empty packages → [ErrMissingValue].
func (c *CoveredTestOutput) TestRun() (TestOutput, error) {
	if c == nil {
		return TestOutput{}, fmt.Errorf("%w: CoveredTestOutput is nil", ErrMissingValue)
	}
	if c.stepID == "" {
		return TestOutput{}, fmt.Errorf("%w: CoveredTestOutput stepID is empty", ErrMissingValue)
	}
	if c.packages.Packages() == "" {
		return TestOutput{}, fmt.Errorf("%w: CoveredTestOutput has empty run-target packages", ErrMissingValue)
	}
	return TestOutput{
		stepID:    c.stepID,
		scope:     c.packages,
		qualifier: c.qualifier,
	}, nil
}

// CoverageOutput is a token proving a Coverage step produced artifacts.
// Zero value is invalid; obtain only from Coverage().
type CoverageOutput struct {
	stepID       string
	qualityScope QualityScope
	qualifier    string
}

// QualityScopeInventoryOutput is an opaque token proving [QualityScopeInventory]
// stored the scoped package inventory artifact and retained the parsed rows for
// later in-process consumers.
type QualityScopeInventoryOutput struct {
	store            *ArtifactStore
	stepID           string
	root             string
	rows             []gatecheck.MutationPackageRow
	schema           int
	format           string
	scopeFingerprint string
}

// MutationScanOutput is a token proving [MutationRunner.Scan] stored gremlins dry-run output
// (mutations.json) in an [ArtifactStore]. It carries correlation identity ([StepID]) and
// [QualityScope]—parsed metrics and durable JSON remain in the store. The token also binds
// the [ArtifactStore] used by the scan producer for artifact reads in [MutationSites] and
// [MutationCoverage]; public callers use only the token, not a separate store parameter.
// It also carries a snapshot of [RunnerOutputMode] from the scan so [MutationSites] and
// [MutationCoverage] can match that run’s silent/verbose error shaping.
type MutationScanOutput struct {
	store        *ArtifactStore
	stepID       string
	qualityScope QualityScope
	pathFilters  mutationPathFilters
	outputMode   OutputMode
	display      stepDisplay
}

// StepID returns the artifact correlation id for the stored gremlins report.
// Invalid token → [ErrMissingValue].
func (o *MutationScanOutput) StepID() (string, error) {
	if o == nil {
		return "", fmt.Errorf("%w: MutationScanOutput is nil", ErrMissingValue)
	}
	if o.stepID == "" {
		return "", fmt.Errorf("%w: MutationScanOutput stepID is empty", ErrMissingValue)
	}
	return o.stepID, nil
}

// QualityScope returns the measurement boundary for this scan.
// Invalid token → [ErrMissingValue].
func (o *MutationScanOutput) QualityScope() (QualityScope, error) {
	if o == nil {
		return QualityScope{}, fmt.Errorf("%w: MutationScanOutput is nil", ErrMissingValue)
	}
	if o.stepID == "" {
		return QualityScope{}, fmt.Errorf("%w: MutationScanOutput stepID is empty", ErrMissingValue)
	}
	if _, err := validateGoTestPackagePattern(qualityScopePackages(o.qualityScope), ErrMissingValue); err != nil {
		return QualityScope{}, fmt.Errorf("%w: MutationScanOutput quality scope packages: %w", ErrMissingValue, err)
	}
	return o.qualityScope, nil
}

// MutationKillsOutput holds gremlins kill stats from [MutationKills] or [MutationRunner.Kill].
// It also carries a snapshot of [RunnerOutputMode] from the kill run so [MutationKillRate] can
// match that run’s silent/verbose error shaping. Accessors return [ErrMissingValue] for nil/empty/incomplete receivers.
type MutationKillsOutput struct {
	stepID       string
	qualityScope QualityScope
	pathFilters  mutationPathFilters
	check        *gatecheck.MutationKillsCheck
	outputMode   OutputMode
	display      stepDisplay
}

func (o *MutationKillsOutput) validateMetricsAccess() error {
	if o == nil {
		return fmt.Errorf("%w: MutationKillsOutput is nil", ErrMissingValue)
	}
	if o.stepID == "" {
		return fmt.Errorf("%w: MutationKillsOutput stepID is empty", ErrMissingValue)
	}
	if o.check == nil {
		return fmt.Errorf("%w: MutationKillsOutput has no check data", ErrMissingValue)
	}
	return nil
}

func (o *MutationKillsOutput) validateScopedMetricsAccess() error {
	if err := o.validateMetricsAccess(); err != nil {
		return err
	}
	if _, err := validateGoTestPackagePattern(qualityScopePackages(o.qualityScope), ErrMissingValue); err != nil {
		return fmt.Errorf("%w: MutationKillsOutput quality scope packages: %w", ErrMissingValue, err)
	}
	return nil
}

func (o *MutationKillsOutput) TotalKilled() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalKilled, nil
}

func (o *MutationKillsOutput) TotalLived() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalLived, nil
}

func (o *MutationKillsOutput) KillRatePercent() (float64, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.KillRatePercent, nil
}

func (o *MutationKillsOutput) TotalNotCovered() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalNotCovered, nil
}

func (o *MutationKillsOutput) TotalTimedOut() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalTimedOut, nil
}

func (o *MutationKillsOutput) TotalNotViable() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalNotViable, nil
}

func (o *MutationKillsOutput) TotalRunnable() (int, error) {
	if err := o.validateMetricsAccess(); err != nil {
		return 0, err
	}
	return o.check.TotalRunnable, nil
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, counter.Add(1))
}
