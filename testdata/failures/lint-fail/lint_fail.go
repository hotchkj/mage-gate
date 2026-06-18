package lintfail

import (
	"fmt"
	"io"
)

func TriggerErrcheckFailure() {
	fmt.Fprintf(io.Discard, "lint failure fixture")
}
