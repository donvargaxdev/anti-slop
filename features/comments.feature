@phase-1
Feature: Comments
  What a person observes from anti-slop comments <pattern>...: a Go comment may only be
  one a tool reads. Code explains itself and tests carry the why, so a doc comment and a
  comment that says why are findings alike. The comments a tool reads are left alone: the
  //go: and //line directives, //nolint:<linter> with a reason after it, a license header, a
  generated-file marker and a mutation annotation. Each finding is one line on stdout,
  path:line:col: comments: <reason>. The command only reports: it exits 0 with findings and
  with none. Its patterns are read as go list reads them, so a pattern that matches no
  package exits 82, and one go list would take for a flag exits 80. The command loads
  packages through the go tool and the disk, so only the binary holds these scenarios.

  @wip @ID-COMMENTS-01 @comments-rule @binary-only
  Scenario: comments reports each comment no tool reads and exits 0
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      package p

      // F does a thing.
      func F() {}
      """
    When anti-slop runs with "comments ./..."
    Then it exits with code 0
    And its standard output says "p.go:3:1: comments: "
    And its standard output is one line

  @wip @ID-COMMENTS-02 @comments-rule @binary-only
  Scenario: comments leaves the comments a tool reads and prints nothing
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      // Copyright 2026 The Authors.

      //go:build !ignore

      package p

      //go:noinline
      func F() {}

      //nolint:gosec // the argument is checked at its edge
      func G() {}
      """
    When anti-slop runs with "comments ./..."
    Then it exits with code 0
    And its standard output is empty
    And its error output is empty

  @wip @ID-COMMENTS-03 @comments-rule @binary-only
  Scenario: comments exits 82 for a pattern that matches no package
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    When anti-slop runs with "comments ./nothing/..."
    Then it exits with code 82
    And its standard output is empty
    And its error output says "anti-slop: the packages cannot be loaded"

  @wip @ID-COMMENTS-04 @comments-rule
  Scenario: comments exits 80 for a pattern that go list would take for a flag
    When anti-slop runs with "comments -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @wip @ID-COMMENTS-05 @comments-rule
  Scenario: comments without a package is a usage failure
    When anti-slop runs with "comments"
    Then it exits with code 80
    And its error output says "comments needs"
