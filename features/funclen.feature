@phase-1
Feature: Function length
  anti-slop funclen reports bodies with more than 70 physical lines between
  their brace lines. Generated files and table-driven case data are excluded.

  @wip @ID-FUNCLEN-01 @funclen-rule @binary-only
  Scenario: funclen reports a long function and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func long() {
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
          _ = 0
      }
      """
    When anti-slop runs with "funclen ./..."
    Then it exits with code 0
    And its standard output lists "p.go:3:1: funclen: function body is 71 lines; limit is 70"
    And its standard output is one line
    And its error output is empty

  @wip @ID-FUNCLEN-02 @funclen-rule
  Scenario: funclen rejects a pattern that go list would take for a flag
    When anti-slop runs with "funclen -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @wip @ID-FUNCLEN-03 @funclen-rule @binary-only
  Scenario: funclen reports when patterns match no package
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "funclen ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"
