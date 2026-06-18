Feature: Quality scope inventory step
  The quality scope inventory step makes package discovery an explicit operation with reusable evidence.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And the output mode is agent

  Scenario: records package inventory for the configured quality scope
    When the gate runs steps qualityscopeinventory
    Then the step passes
    And the artifact store contains "quality-scope-package-rows.json"
    And the artifact provenance records tool "go list -e"
    And the artifact provenance records the configured scope
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./zz_bdd_not_default/...
      """

  Scenario: quality-scope tags affect inventory discovery
    Given the quality scope has build tag "mage"
    And the quality scope has build tag "integration"
    When the gate runs steps qualityscopeinventory
    Then the step passes
    And the command `go` is run with arguments:
      """
      list
      -e
      -tags=mage,integration
      -f
      <package-inventory-format>
      ./zz_bdd_not_default/...
      """

  Scenario: quality-scope excludes do not narrow inventory discovery
    Given the quality scope excludes "testutil"
    When the gate runs steps qualityscopeinventory
    Then the step passes
    And the command `go` is run with arguments:
      """
      list
      -e
      -f
      <package-inventory-format>
      ./zz_bdd_not_default/...
      """
