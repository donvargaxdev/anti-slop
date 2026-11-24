@phase-1
Feature: Comments
  What a person observes from anti-slop comments <pattern>...: a Go comment may only be
  one a tool reads. Code explains itself and tests carry the why, so a doc comment and a
  comment that says why are findings alike. The comments a tool reads are left alone: the
  //go: and //line directives, //nolint:<linter> with a reason after it, a license header, a
  generated-file marker and a mutation annotation. Source attribution notices, cgo
  preambles and cgo export directives are also preserved. Each finding is one line on stdout,
  path:line:col: comments: <reason>. The command only reports: it exits 0 with findings and
  with none. Its patterns are read as go list reads them, so a pattern that matches no
  package exits 82, and one go list would take for a flag exits 80. The command loads
  packages through the go tool and the disk, so only the binary holds these scenarios.

  @ID-COMMENTS-01 @comments-rule @binary-only
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

  @ID-COMMENTS-02 @comments-rule @binary-only
  Scenario: comments leaves the comments a tool reads and prints nothing
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      //go:build !ignore
      // +build !ignore

      // Copyright 2026 The Authors.

      // Compile-first mechanics adapted from itos-container T-10, f6f339c (AGPL-3.0).
      // Ownership/linked graph probes follow itos-cc's architecture-check design.
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

  @ID-COMMENTS-03 @comments-rule @binary-only
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

  @ID-COMMENTS-04 @comments-rule
  Scenario: comments exits 80 for a pattern that go list would take for a flag
    When anti-slop runs with "comments -- -x"
    Then it exits with code 80
    And its error output says "-x"

  @ID-COMMENTS-05 @comments-rule
  Scenario: comments without a package is a usage failure
    When anti-slop runs with "comments"
    Then it exits with code 80
    And its error output says "comments needs"

  @ID-COMMENTS-06 @comments-rule @binary-only
  Scenario: comments leaves cgo export directives and reports adjacent Go documentation
    Given the file "go.mod" holds:
      """
      module example.test/m

      go 1.27
      """
    And the file "p.go" holds:
      """
      //go:build cgo

      // Copyright 2026 The Authors.
      package p

      // #include <stdlib.h>
      import "C"

      // F is callable from C.
      //export F
      func F() {}
      """
    When anti-slop runs with "comments ./..." and the environment "CGO_ENABLED=1"
    Then it exits with code 0
    And its standard output says "p.go:9:1: comments: "
    And its standard output is one line
    And its error output is empty
