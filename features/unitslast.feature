@phase-1
Feature: Units and qualifiers last
  anti-slop unitslast <pattern>... reports Go declarations with a leading
  unit or qualifier. Each finding occupies one line on stdout and exits 0.

  @ID-UNITSLAST-01 @unitslast-rule @binary-only
  Scenario: unitslast reports a leading unit and qualifier and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      var maxLatencyMs int
      var msTimeout int
      """
    When anti-slop runs with "unitslast ./..."
    Then it exits with code 0
    And its standard output lists "p.go:3:5: unitslast: move max to the end of maxLatencyMs"
    And its standard output lists "p.go:4:5: unitslast: move ms to the end of msTimeout"
    And its error output is empty

  @ID-UNITSLAST-02 @unitslast-rule @binary-only
  Scenario: unitslast leaves trailing units and qualifiers
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      var latencyMsMax int
      var timeoutMs int
      """
    When anti-slop runs with "unitslast ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @ID-UNITSLAST-03 @unitslast-rule
  Scenario: unitslast rejects a pattern that go list would take for a flag
    When anti-slop runs with "unitslast -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-UNITSLAST-04 @unitslast-rule @binary-only
  Scenario: unitslast reports when packages cannot be loaded
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "unitslast ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"

  @ID-UNITSLAST-05 @unitslast-rule
  Scenario: unitslast help describes its finding format
    When anti-slop runs with "unitslast --help"
    Then it exits with code 0
    And its standard output says "path:line:col: unitslast: <reason>"
    And its error output is empty

  @ID-UNITSLAST-06 @unitslast-rule @binary-only
  Scenario: unitslast recognizes binary units with their standard capitalization
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      var KiB_read int
      var MiBReceived int
      var GiBHTTPResponse int
      var receivedMiB int
      var MiByte int
      """
    When anti-slop runs with "unitslast ./..."
    Then it exits with code 0
    And its standard output lists "p.go:3:5: unitslast: move kib to the end of KiB_read"
    And its standard output lists "p.go:4:5: unitslast: move mib to the end of MiBReceived"
    And its standard output lists "p.go:5:5: unitslast: move gib to the end of GiBHTTPResponse"
    And its standard output does not say "end of receivedMiB"
    And its standard output does not say "end of MiByte"
    And its error output is empty
