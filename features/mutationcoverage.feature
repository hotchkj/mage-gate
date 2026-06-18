Feature: MutationCoverage step
  The mutation coverage step verifies that dry-run mutation coverage meets the configured threshold.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a mutation threshold of 2 sites
    And the module "example.com/mod" has package "pkg" at "/mod/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
    And the tool spec for "mutationcoverage" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
    And the tool spec for "mutationscan" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

  Scenario: passes when mutation coverage meets threshold
    Given a mutation coverage min of 80 percent
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step passes

  Scenario: fails when mutation coverage is below threshold
    Given a mutation coverage min of 90 percent
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step fails
    And the error is ErrMutationCoverageFailed
    And the output is an ERROR/Fix/Hint diagnostic

  Scenario: disabled threshold passes regardless
    Given a mutation coverage min of 0 percent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step passes

  Scenario: quality scope excludes narrow the mutation boundary
    Given a mutation coverage min of 80 percent
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/zz_bdd_not_default/pkg
      --exclude-files=^zz_bdd_not_default/testutil(/|$)
      --dry-run
      """
    And the mutation-coverage evaluation did not include "zz_bdd_not_default/testutil/pkg.go"

  Scenario: test file patterns are excluded from mutation metrics
    Given a mutation coverage min of 80 percent
    And the quality scope test file patterns include "*_test.go"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step passes
    And the mutation-coverage evaluation did not include "pkg/foo_test.go"

  Scenario: configured quality scope narrows mutation coverage command input
    Given a mutation coverage min of 80 percent
    And the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationcoverage
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app
      --dry-run
      """
