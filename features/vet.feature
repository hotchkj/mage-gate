Feature: Vet step
  The vet step runs go vet on the configured packages.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."

  Scenario Outline: passes on clean code
    Given the output mode is <mode>
    When the gate runs steps vet
    Then the step passes
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is empty                        |
      | verbose | the output contains the tool's full output |

  Scenario Outline: fails when vet issues exist
    Given the codebase has vet issues
    And the output mode is <mode>
    When the gate runs steps vet
    Then the step fails
    And <output>

    Examples:
      | mode    | output                                     |
      | agent   | the output is an ERROR/Fix/Hint diagnostic |
      | verbose | the output contains the tool's full output |

  Scenario: passes vet extra args through
    Given the output mode is agent
    And vet has an extra argument of "arg1" specified
    When the gate runs steps vet
    Then the step passes
    And the command `go` is run with arguments:
      """
      vet
      arg1
      ./zz_bdd_not_default/...
      """

  Scenario: uses the configured package scope
    Given the package scope is "./cmd/..."
    And the output mode is agent
    When the gate runs steps vet
    Then the step passes
    And the command `go` is run with arguments:
      """
      vet
      ./cmd/...
      """
