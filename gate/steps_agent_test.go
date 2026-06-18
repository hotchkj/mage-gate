// Vision: Agent library mode—compile and vet succeed in silent display without leaking stdout to the display writer.
package gate

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	"github.com/hotchkj/mage-gate/gatetest"
)

func TestAgentModeLibrarySilentOnSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	mem := gatetest.NewMemoryFileOps()
	fakeRoot := "/fake-root"
	inner := gateStepFakeRunner(mem,
		cmdtest.On("go build", gatetest.NoopCommand),
		cmdtest.On("go vet", gatetest.NoopCommand),
	)
	wrapped := mustNewDisplayRunner(t, inner, OutputModeAgent, &buf, io.Discard)
	pkgScope := mustNewPackageScope(t, "./...")
	if err := Compile(context.Background(), wrapped, mem, fakeRoot, pkgScope); err != nil {
		t.Fatal(err)
	}
	if err := Vet(context.Background(), wrapped, mem, fakeRoot, pkgScope); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "Compile...\nVet..." {
		t.Fatalf("expected start lines for compile/vet in silent display, got: %q", buf.String())
	}
}
