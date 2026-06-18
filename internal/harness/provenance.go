// Vision: Provenance metadata stamped on artifacts so later steps can audit which tool/version wrote a file.
package harness

// Provenance records the origin of a stored artifact.
// Packages is the go command target (for example the go test run pattern), not necessarily
// the coverpkg measurement seed used to build -coverpkg.
type Provenance struct {
	StepID   string
	Tool     string
	Packages string
}
