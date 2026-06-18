Feature: MutationSites step
  The mutation sites step verifies that no file exceeds the mutation sites threshold.
  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a mutation threshold of 2 sites
    And the module "example.com/mod" has package "pkg" at "/mod/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
    And the tool spec for "mutationsites" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"
    And the tool spec for "mutationscan" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

  Scenario Outline: passes when below threshold
    Given the output mode is <mode>
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when above threshold
    Given the codebase has excessive mutation sites
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  # execution-specific dry-run concerns move to operation-oriented scan scenarios

  Scenario: uses the configured quality scope for mutation analysis
    Given the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites
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

  Scenario: quality scope excludes narrow the mutation boundary
    Given the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites
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
    And the mutation-sites report did not include "zz_bdd_not_default/testutil/pkg.go"

  Scenario: test file patterns are excluded from mutation metrics
    Given the quality scope test file patterns include "*_test.go"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites
    Then the step passes
    And the mutation-sites report did not include "pkg/foo_test.go"
