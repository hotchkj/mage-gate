Feature: Compile step
  The compile step verifies packages compile with go build.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."

  Scenario Outline: passes when the codebase compiles
    Given the codebase compiles cleanly
    And the output mode is <mode>
    When the gate runs steps compile
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when the codebase does not compile
    Given the codebase fails to compile
    And the output mode is <mode>
    When the gate runs steps compile
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: passes compile extra args through
    Given the codebase compiles cleanly
    And the output mode is agent
    And compile has an extra argument of "arg1" specified
    When the gate runs steps compile
    Then the step passes
    And the command `go` is run with arguments:
      """
      build
      arg1
      ./zz_bdd_not_default/...
      """

  Scenario: uses the configured package scope
    Given the codebase compiles cleanly
    And the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps compile
    Then the step passes
    And the command `go` is run with arguments:
      """
      build
      ./cmd/...
      """
