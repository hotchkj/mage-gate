//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/mage-gate/cmdtest"
	qg "github.com/hotchkj/mage-gate/gate"
	"github.com/hotchkj/mage-gate/gatetest"
)

const (
	mageFlowModule       = "github.com/hotchkj/mage-gate"
	mageFlowRoot         = "/fake-root"
	mageFlowPackage      = "github.com/hotchkj/mage-gate/gate"
	mageFlowLintSpec     = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"
	mageFlowDeadSpec     = "golang.org/x/tools/cmd/deadcode@v0.31.0"
	mageFlowMarkdownSpec = "github.com/shinagawa-web/gomarklint/v3@v3.2.3"
	mageFlowGocycloSpec  = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"
	mageFlowGremlinsSpec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
)

func mageFlowConfig() *config {
	coverage := 90.0
	crap := 8.0
	duration := 1.0
	sites := 50
	return &config{
		Thresholds: thresholdConfig{
			CoverageMin:      &coverage,
			CrapMax:          &crap,
			DurationMax:      &duration,
			MutationSitesMax: &sites,
		},
		Lint:     lintConfig{Config: ".golangci.yml", ToolSpec: mageFlowLintSpec},
		Deadcode: deadcodeConfig{ToolSpec: mageFlowDeadSpec},
		Markdownlint: markdownlintConfig{
			ToolSpec: mageFlowMarkdownSpec,
			Args:     []string{"--config", ".gomarklint.json"},
		},
		Crap:     crapConfig{ToolSpec: mageFlowGocycloSpec},
		Gremlins: gremlinsConfig{ToolSpec: mageFlowGremlinsSpec},
	}
}

func mageFlowPolicyTOML() []byte {
	return []byte(`[thresholds]
coverage_min = 90.0
crap_max = 8.0
duration_max = 1.0
mutation_sites_max = 50
mutation_kills_min_rate = 90
mutation_coverage_min = 50

[lint]
config = ".golangci.yml"
tool_spec = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

[quality_scope]
packages = "./..."
tags = ["mage"]

[deadcode]
tool_spec = "golang.org/x/tools/cmd/deadcode@v0.31.0"

[markdownlint]
tool_spec = "github.com/shinagawa-web/gomarklint/v3@v3.2.3"
args = ["--config", ".gomarklint.json"]

[crap]
tool_spec = "github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0"

[gremlins]
tool_spec = "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

[unittests]
shuffle = false

[integrationtests]
packages = ""
shuffle = false
`)
}

var mageFlowGremlinsReport = []byte(`{"files":[{"file_name":"pkg/foo.go","package":"p","mutations":[
	{"status":"KILLED"},
	{"status":"KILLED"},
	{"status":"NOT_COVERED"}
]}]}`)

func mageFlowScopes(t *testing.T) (qg.QualityScope, qg.PackageScope) {
	t.Helper()
	qualityScope, err := qg.NewQualityScope("./...")
	if err != nil {
		t.Fatalf("NewQualityScope: %v", err)
	}
	pkgScope, err := qg.NewPackageScope("./...")
	if err != nil {
		t.Fatalf("NewPackageScope: %v", err)
	}
	return qualityScope, pkgScope
}

func mageFlowRunner(t *testing.T, mem *gatetest.MemoryFileOps) qg.CommandRunner {
	t.Helper()
	base := []cmdtest.RunnerOption{
		cmdtest.On("go test", gatetest.GoTestPass(mem, mageFlowPackage)),
		cmdtest.On("go tool cover", gatetest.GoToolCoverFunc(map[string]float64{
			"file.go:1:\tTested": 100,
		}, 100.0)),
		cmdtest.On("go list", gatetest.GoList(mageFlowModule, mageFlowRoot, map[string]gatetest.PackageListInfo{
			mageFlowPackage: gatetest.DirOnly(filepath.Join(mageFlowRoot, "gate")),
		})),
		cmdtest.On("go build", gatetest.NoopCommand),
		cmdtest.On("go vet", gatetest.NoopCommand),
		cmdtest.On("golangci-lint", gatetest.NoopCommand),
		cmdtest.On("deadcode", gatetest.NoopCommand),
		cmdtest.On("gomarklint", gatetest.NoopCommand),
		cmdtest.On("gocyclo", gatetest.Gocyclo(map[string]int{"Tested": 1})),
		cmdtest.On("gremlins", gatetest.Gremlins(mem, "", mageFlowGremlinsReport)),
	}
	inner := cmdtest.NewFakeRunner(base...)
	runner, err := qg.NewDisplayRunner(inner, qg.OutputModeAgent, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("NewDisplayRunner: %v", err)
	}
	return runner
}

func useMageFlowRuntime(t *testing.T, mem *gatetest.MemoryFileOps) {
	t.Helper()
	if err := mem.Root("."); err != nil {
		t.Fatalf("Root: %v", err)
	}
	origReadPolicyFile := readPolicyFile
	origNewRunner := newRunner
	origNewResolver := newResolver
	origNewFileOps := newFileOps
	origNewArtifactStore := newArtifactStore
	runner := mageFlowRunner(t, mem)
	resolver := gatetest.NewFakeToolResolver().SetDefaultToLocal(true)
	readPolicyFile = func(string) ([]byte, error) { return mageFlowPolicyTOML(), nil }
	newRunner = func() (qg.CommandRunner, error) { return runner, nil }
	newResolver = func() qg.ToolResolver { return resolver }
	newFileOps = func() qg.FileOps { return mem.Fork() }
	newArtifactStore = qg.NewArtifactStore
	t.Cleanup(func() {
		readPolicyFile = origReadPolicyFile
		newRunner = origNewRunner
		newResolver = origNewResolver
		newFileOps = origNewFileOps
		newArtifactStore = origNewArtifactStore
	})
}

func TestRunGateStepsRunsPreTestStages(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	runner := mageFlowRunner(t, mem)
	resolver := gatetest.NewFakeToolResolver().SetDefaultToLocal(true)
	_, pkgScope := mageFlowScopes(t)

	err := runGateSteps(
		context.Background(),
		runner,
		resolver,
		mem,
		mageFlowRoot,
		pkgScope,
		mageFlowConfig(),
	)
	if err != nil {
		t.Fatalf("runGateSteps: %v", err)
	}
}

func TestRunGateStepsSkipsMarkdownlintWhenDisabled(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	runner := mageFlowRunner(t, mem)
	resolver := gatetest.NewFakeToolResolver().SetDefaultToLocal(true)
	_, pkgScope := mageFlowScopes(t)
	cfg := mageFlowConfig()
	cfg.Markdownlint = markdownlintConfig{}

	err := runGateSteps(
		context.Background(),
		runner,
		resolver,
		mem,
		mageFlowRoot,
		pkgScope,
		cfg,
	)
	if err != nil {
		t.Fatalf("runGateSteps without markdownlint: %v", err)
	}
}

func TestRunMarkdownLintUsesConfiguredFakes(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	useMageFlowRuntime(t, mem)
	if err := runMarkdownLint(); err != nil {
		t.Fatalf("runMarkdownLint: %v", err)
	}
	if err := MarkdownLint(); err != nil {
		t.Fatalf("MarkdownLint wrapper: %v", err)
	}
}

func TestRunGatePostTestRunsDurationCoverageAndCrap(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	runner := mageFlowRunner(t, mem)
	resolver := gatetest.NewFakeToolResolver().SetDefaultToLocal(true)
	store := qg.NewArtifactStore()
	qualityScope, pkgScope := mageFlowScopes(t)
	cfg := mageFlowConfig()
	ctx := context.Background()

	inv, err := runQualityScopeInventoryPhase(ctx, runner, store, mem, mageFlowRoot, qualityScope)
	if err != nil {
		t.Fatalf("QualityScopeInventory: %v", err)
	}
	unitCov, err := runCoverageTestPhase(
		ctx, runner, store, mem, cfg, pkgScope, qualityScope, &inv, mageFlowRoot,
	)
	if err != nil {
		t.Fatalf("runCoverageTestPhase: %v", err)
	}
	err = runGatePostTest(ctx, runner, resolver, store, mem, mageFlowRoot, cfg, &unitCov, &inv)
	if err != nil {
		t.Fatalf("runGatePostTest: %v", err)
	}
}

func TestRunIntegrationPassSkipsWhenOptionalAndUnconfigured(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	err := runIntegrationPass(
		context.Background(),
		mageFlowRunner(t, mem),
		qg.NewArtifactStore(),
		mem,
		mageFlowRoot,
		mageFlowConfig(),
		false,
	)
	if err != nil {
		t.Fatalf("runIntegrationPass optional skip: %v", err)
	}
}

func TestRunIntegrationPassRequiresConfiguredPackages(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	err := runIntegrationPass(
		context.Background(),
		mageFlowRunner(t, mem),
		qg.NewArtifactStore(),
		mem,
		mageFlowRoot,
		mageFlowConfig(),
		true,
	)
	if err == nil {
		t.Fatal("expected error for required integration packages")
	}
	if !errors.Is(err, errIntegrationNoPackages) {
		t.Fatalf("expected errIntegrationNoPackages, got %v", err)
	}
}

func TestRunIntegrationPassRunsConfiguredPackages(t *testing.T) {
	t.Parallel()
	mem := gatetest.NewMemoryFileOps()
	cfg := mageFlowConfig()
	cfg.Integrationtests.Packages = "./integration/..."
	cfg.Integrationtests.Tags = "integration"

	err := runIntegrationPass(
		context.Background(),
		mageFlowRunner(t, mem),
		qg.NewArtifactStore(),
		mem,
		mageFlowRoot,
		cfg,
		true,
	)
	if err != nil {
		t.Fatalf("runIntegrationPass configured: %v", err)
	}
}

func TestStandaloneMageTargetsUseConfiguredFakes(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	useMageFlowRuntime(t, mem)

	targets := []struct {
		name string
		run  func() error
	}{
		{name: "lint", run: runLint},
		{name: "format", run: runFormat},
		{name: "deadcode", run: runDeadcode},
		{name: "markdownlint", run: runMarkdownLint},
		{name: "vet", run: runVet},
		{name: "compile", run: runCompile},
		{name: "unittests", run: runUnittests},
		{name: "mutationsites", run: runMutationSites},
		{name: "mutationcoverage", run: runMutationCoverage},
		{name: "coverage", run: runCoverage},
		{name: "crap", run: runCrap},
		{name: "duration", run: runDuration},
		{name: "mutationkills", run: runMutationKills},
		{name: "gate", run: runGate},
	}
	for _, target := range targets {
		if err := target.run(); err != nil {
			t.Fatalf("%s target: %v", target.name, err)
		}
	}

	wrappers := []struct {
		name string
		run  func() error
	}{
		{name: "Gate", run: Gate},
		{name: "Lint", run: Lint},
		{name: "Format", run: Format},
		{name: "Deadcode", run: Deadcode},
		{name: "MarkdownLint", run: MarkdownLint},
		{name: "Vet", run: Vet},
		{name: "Compile", run: Compile},
		{name: "Unittests", run: Unittests},
		{name: "MutationSites", run: MutationSites},
		{name: "MutationCoverage", run: MutationCoverage},
		{name: "Coverage", run: Coverage},
		{name: "Crap", run: Crap},
		{name: "Duration", run: Duration},
		{name: "MutationKills", run: MutationKills},
	}
	for _, wrapper := range wrappers {
		if err := wrapper.run(); err != nil {
			t.Fatalf("%s wrapper: %v", wrapper.name, err)
		}
	}
}

func TestIntegrationtestsWrapperRunsConfiguredPackages(t *testing.T) {
	mem := gatetest.NewMemoryFileOps()
	useMageFlowRuntime(t, mem)
	readPolicyFile = func(string) ([]byte, error) {
		policy := strings.Replace(
			string(mageFlowPolicyTOML()),
			"packages = \"\"\nshuffle = false",
			"packages = \"./integration/...\"\ntags = \"integration\"\nshuffle = false",
			1,
		)
		return []byte(policy), nil
	}

	if err := Integrationtests(); err != nil {
		t.Fatalf("Integrationtests wrapper: %v", err)
	}
}
