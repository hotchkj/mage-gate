Feature: Mutation runner scan
  The mutation runner scan operation produces mutation artifacts for downstream checks.

  Background:
    Given the package scope is "./..."
    And the quality scope is "./..."
    And the module "example.com/mod" has package "example.com/mod/internal/app" at "/mod/internal/app"
    And the module "example.com/mod" has package "example.com/mod/internal/testutil" at "/mod/internal/testutil"
    And the module "example.com/mod" has package "example.com/mod/vendor/lib" at "/mod/vendor/lib"
    And the tool spec for "mutationscan" is "github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1"

  Scenario Outline: tool resolution for gremlins dry-run
    Given the local tool for "mutationscan" is "<state>"
    When the gate runs steps qualityscopeinventory, mutationscan
    Then <outcome>
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./...
      """
    And the command `<command>` is run with arguments:
      """
      <arguments>
      """

    Examples:
      | state       | outcome                   | command  | arguments                                                                                                                                                                                                             |
      | matching    | the tool runs locally     | gremlins | unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib --dry-run                                                                   |
      | mismatched  | the tool runs via go run  | go       | run github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1 unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib --dry-run |
      | missing     | the tool runs via go run  | go       | run github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1 unleash -o <artifact>/mutations.json --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib --dry-run |
      | unprobeable | the tool is not available | none     |                                                                                                                                                                                                                       |

  Scenario: passes mutation extra args through
    Given the output mode is agent
    And mutation has an extra argument of "arg1" specified
    When the gate runs steps qualityscopeinventory, mutationscan
    Then mutation scan succeeds
    And the command `go` is run with arguments:
      """
      run
      github.com/hotchkj/gremlins/cmd/gremlins@v0.6.1-pre.1
      unleash
      -o
      <artifact>/mutations.json
      --coverpkg=example.com/mod/internal/app,example.com/mod/internal/testutil,example.com/mod/vendor/lib
      --dry-run
      arg1
      """

  Scenario: scan output can feed multiple downstream mutation checks
    Given a mutation threshold of 50 sites
    And a mutation coverage min of 80 percent
    When the gate runs steps qualityscopeinventory, mutationscan, mutationsites, mutationcoverage
    Then the step passes
    And the gremlins dry-run command ran once
