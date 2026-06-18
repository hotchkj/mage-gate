// Vision: Exported test hooks for packages that cannot import harness internals (export_test pattern).
package harness

func ArtifactSubdirForTest(h *StepRunner) string {
	return h.artifactSubdir
}

// ArtifactLogicalDirForTest returns the canonical logical artifact directory wired during NewStepRunner.
func ArtifactLogicalDirForTest(h *StepRunner) string {
	return h.artifacts.Dir()
}
