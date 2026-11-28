@phase-1
Feature: Recursion
  anti-slop recursion <pattern>... reports named functions and methods that call
  themselves through one to three statically resolved calls in the same package.
  Each finding occupies one line on stdout and the command exits 0.

  @ID-RECURSION-01 @recursion-rule @binary-only
  Scenario: recursion reports each function in a cycle and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func direct() { direct() }
      func first() { second() }
      func second() { first() }
      func caller() { first() }
      """
    And the file "cycle.go" holds:
      """
      package p

      func alpha() { beta() }
      func beta() { gamma() }
      """
    And the file "cycle_end.go" holds:
      """
      package p

      func gamma() { alpha() }
      """
    When anti-slop runs with "recursion ./..."
    Then its standard output lists "p.go:3:6: recursion: direct calls itself"
    And its standard output lists "p.go:4:6: recursion: first calls itself through a cycle"
    And its standard output lists "p.go:5:6: recursion: second calls itself through a cycle"
    And its standard output lists "cycle.go:3:6: recursion: alpha calls itself through a cycle"
    And its standard output lists "cycle.go:4:6: recursion: beta calls itself through a cycle"
    And its standard output lists "cycle_end.go:3:6: recursion: gamma calls itself through a cycle"
    And its standard output does not say "caller"
    And it exits with code 0
    And its error output is empty

  @ID-RECURSION-02 @recursion-rule @binary-only
  Scenario: recursion leaves a call chain that ends and a cycle of four calls
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      func first() { second() }
      func second() {}

      func alpha() { beta() }
      func beta() { gamma() }
      func gamma() { delta() }
      func delta() { alpha() }
      """
    When anti-slop runs with "recursion ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-RECURSION-03 @recursion-rule
  Scenario: recursion rejects a pattern that go list would take for a flag
    When anti-slop runs with "recursion -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-RECURSION-04 @recursion-rule @binary-only
  Scenario: recursion reports when packages cannot be loaded
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "recursion ./nothing/..."
    Then its error output says "anti-slop: the packages cannot be loaded"
    And it exits with code 82
    And its standard output is empty
