Feature: Artifact provenance
  The artifact store records what each step produced, with what tool,
  for what scope. Downstream steps and agents query this provenance.

  Background:
    Given the package scope is "./zz_bdd_not_default/..."
    And the quality scope is "./zz_bdd_not_default/..."
    And the module "example.com/mod" has package "example.com/mod/zz_bdd_not_default/pkg" at "/mod/zz_bdd_not_default/pkg"
    And a coverage threshold of 90
    And the output mode is agent

  Scenario: test step records test events provenance
    Given the codebase has 95% test coverage
    When the gate runs steps test
    Then the artifact store contains "test-events.jsonl"
    And the artifact provenance records tool "go test -json"
    And the artifact provenance records the configured scope

  Scenario: instrumentation pass records coverage profile provenance
    Given the codebase has 95% test coverage
    When the gate runs steps qualityscopeinventory, coveredtest
    Then the artifact store contains "coverage.out"
    And the artifact provenance records tool "go test -coverprofile"
    And the artifact provenance records the configured scope

  Scenario: coverage step reads upstream and produces its own artifact
    Given the codebase has 95% test coverage
    When the gate runs steps qualityscopeinventory, coveredtest, coverage
    Then the artifact store contains "coverage.out" from the "coverage" step
    And the artifact store contains "coverage.out" from the "coveredtest" step

  Scenario: provenance records a non-default configured scope
    Given the codebase has 95% test coverage
    And the package scope is "./cmd/..."
    When the gate runs steps test
    Then the artifact store contains "test-events.jsonl"
    And the artifact provenance records the configured scope
