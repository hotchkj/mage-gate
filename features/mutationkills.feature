Feature: MutationKills step
  Callers can run MutationKills to evaluate whether tests kill generated mutants.
  This on-demand check is separate from the precommit MutationSites budget.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And a mutation kills min rate of 80 percent
    And the module "example.com/mod" has package "pkg" at "/mod/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/testutil" at "/mod/zz_bdd_not_default/testutil"
    And the tool spec for "mutationkills" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

  Scenario Outline: passes when kill rate meets threshold
    Given the output mode is <mode>
    And the mutation test result has 8 killed and 2 lived mutations
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when kill rate below threshold
    Given a mutation kills min rate of 90 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step fails
    And the error is ErrMutationKillsFailed
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when no killed or lived results exist
    Given a mutation kills min rate of 80 percent
    And the mutation test result has 0 killed and 0 lived mutations
    And the output mode is <mode>
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step fails
    And the error is ErrMutationKillsFailed
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: disabled threshold passes regardless
    Given a mutation kills min rate of 0 percent
    And the mutation test result has 0 killed and 10 lived mutations
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes

  Scenario Outline: handles mixed mutation statuses
    Given the output mode is <mode>
    And the mutation test result has mixed statuses:
      | status      | count |
      | KILLED      | 8     |
      | LIVED       | 2     |
      | NOT_COVERED | 3     |
      | TIMED_OUT   | 1     |
      | NOT_VIABLE  | 1     |
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario: mutations.json records a survivor file when mutations lived
    Given a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And the mutations artifact has 1 survivor file

  Scenario: mutations.json records no survivor files when all mutations killed
    Given a mutation kills min rate of 0 percent
    And the mutation test result has 10 killed and 0 lived mutations
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And the mutations artifact has no survivor files

  Scenario: passes mutation extra args through
    Given the output mode is agent
    And the mutation test result has 8 killed and 2 lived mutations
    And mutation has an extra argument of "arg1" specified
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil
      arg1
      """

  Scenario: uses the configured quality scope for mutation analysis
    Given the mutation test result has 8 killed and 2 lived mutations
    And the quality scope is "./internal/..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app
      """

  Scenario: quality scope excludes narrow the mutation boundary
    Given the mutation test result has 8 killed and 2 lived mutations
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
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
      """
    And the mutation-kills evaluation did not include "zz_bdd_not_default/testutil/pkg.go"

  Scenario: mutation kill rejects generic build-tag arguments
    Given a mutation kills min rate of 0 percent
    And mutation has an extra argument of "--tags=consumer" specified
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step fails
    And the error is ErrInvalidOption
    And the step does not dispatch any commands

  Scenario Outline: tool resolution for gremlins
    Given a mutation kills min rate of 75 percent
    And the module "example.com/mod" has package "example.com/mod/pkg" at "/mod/pkg"
    And the local tool for "mutationkills" is "<state>"
    When the gate runs steps qualityscopeinventory, mutationkills
    Then <outcome>
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./zz_bdd_not_default/...
      """
    And the command `<command>` is run with arguments:
      """
      <arguments>
      """

    Examples:
      | state       | outcome                   | command  | arguments                                                                                                                                                                                            |
      | matching    | the tool runs locally     | gremlins | unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil                                                                   |
      | mismatched  | the tool runs via go run  | go       | run github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1 unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil |
      | missing     | the tool runs via go run  | go       | run github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1 unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/zz_bdd_not_default/pkg,example.com/mod/zz_bdd_not_default/testutil |
      | unprobeable | the tool is not available | none     |                                                                                                                                                                                                      |

  Scenario: test file patterns are excluded from mutation metrics
    Given the mutation test result has 8 killed and 2 lived mutations
    And the quality scope test file patterns include "*_test.go"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    Then the step passes
    And the mutation-kills evaluation did not include "pkg/foo_test.go"

  Scenario: full-run output can satisfy mutation coverage without dry-run
    Given a mutation coverage min of 80 percent
    And a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation coverage is evaluated from full-run artifacts
    Then the step passes
    And the gremlins dry-run command did not run

  Scenario: full-run output fails mutation coverage when below threshold
    Given a mutation coverage min of 95 percent
    And a mutation kills min rate of 0 percent
    And the mutation test result has mixed statuses:
      | status      | count |
      | KILLED      | 8     |
      | LIVED       | 2     |
      | NOT_COVERED | 20    |
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation coverage is evaluated from full-run artifacts
    Then the step fails
    And the error is ErrMutationCoverageFailed

  Scenario: full-run mutation coverage adapter honors quality scope excludes
    Given a mutation coverage min of 80 percent
    And a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation coverage is evaluated from full-run artifacts
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
      """
    And the mutation-coverage evaluation did not include "zz_bdd_not_default/testutil/pkg.go"

  Scenario: full-run mutation coverage adapter excludes test file patterns
    Given a mutation coverage min of 80 percent
    And a mutation kills min rate of 0 percent
    And the quality scope test file patterns include "*_test.go"
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation coverage is evaluated from full-run artifacts
    Then the step passes
    And the mutation-coverage evaluation did not include "pkg/foo_test.go"

  Scenario: full-run output can satisfy mutation sites without dry-run
    Given a mutation threshold of 50 sites
    And a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation sites are evaluated from full-run artifacts
    Then the step passes
    And the gremlins dry-run command did not run

  Scenario: full-run output fails mutation sites when above threshold
    Given a mutation threshold of 1 sites
    And a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation sites are evaluated from full-run artifacts
    Then the step fails
    And the error is ErrMutationSitesFailed

  Scenario: full-run mutation sites adapter honors quality scope excludes
    Given a mutation threshold of 50 sites
    And a mutation kills min rate of 0 percent
    And the mutation test result has 8 killed and 2 lived mutations
    And the quality scope excludes "testutil"
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation sites are evaluated from full-run artifacts
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
      """
    And the mutation-sites evaluation did not include "zz_bdd_not_default/testutil/pkg.go"

  Scenario: full-run mutation sites adapter excludes test file patterns
    Given a mutation threshold of 50 sites
    And a mutation kills min rate of 0 percent
    And the quality scope test file patterns include "*_test.go"
    And the mutation test result has 8 killed and 2 lived mutations
    And the output mode is agent
    When the gate runs steps qualityscopeinventory, mutationkills
    And mutation sites are evaluated from full-run artifacts
    Then the step passes
    And the mutation-sites evaluation did not include "pkg/foo_test.go"
