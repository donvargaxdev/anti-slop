// Package command is the comments command, in the comments slice: its kong
// struct and Run are the UI, and its Query and Handler the application.
//
// Run checks the command line's patterns into Patterns, the one place they
// are checked, hands them to Handle with the context cmd made, and writes
// each finding as a line. The command is report-only: findings are its
// output, never its failure, so it exits 0 with any number of them. A failure
// is the port's, and Classify, beside it, gives it its problem.
package command

import (
	"context"
	"fmt"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
)

// CLI is the comments command's command line: the packages it checks.
type CLI struct {
	Patterns []string `arg:"" help:"The packages to check, as go list reads them: ./..., ./internal/..., a directory or an import path."`
}

// Help is comments' own help: what it prints and every exit code it can
// return. The wording is what the help must hold, because a scenario checks
// it and a script reads it.
//
// The block is indented, so it is preformatted and holds its lines whole at
// any terminal width; only the sentence above it is re-wrapped.
func (CLI) Help() string {
	return "\nA Go comment may only be one a tool reads: a //go: or //line directive, a //nolint:<linter> with a reason, " +
		"a license header, a generated-file marker or a mutation annotation. Every other comment is a finding. " +
		"The command only reports: it exits 0 with findings.\n\n" +
		"    Each finding is one line on stdout: path:line:col: comments: <reason>.\n" +
		"    There is no --json output yet.\n" +
		"    Exit codes:\n" +
		"      0  success, with or without findings\n" +
		"      80  the command line is wrong\n" +
		"      82  the packages cannot be loaded\n" +
		"      100  an error nothing classified, a bug\n" +
		"      130  interrupted\n"
}

// Query is what the comments command asks of the checker: the packages to
// check, as they were checked.
type Query struct {
	Patterns Patterns
}

// Handler is the comments command's application: the Checker port, given by
// cmd, which chooses the adapter behind it. No value of a command line is
// read here.
type Handler struct {
	Checker port.Checker
}

// Handle checks the packages q names through the Checker port, passing ctx on
// to it. It only wires: the rule is the checker's.
func (h Handler) Handle(ctx context.Context, q Query) ([]port.Finding, error) {
	found, failure := h.Checker.Check(ctx, q.Patterns.List())
	if failure != nil {
		return nil, failure
	}
	return found, nil
}

// Run checks the packages the command names, through h and with the context
// cmd made, writes each finding on its own line of stdout, or reports the
// failure it met, and returns the exit code the UI gives it.
func (c CLI) Run(ctx context.Context, ui *cli.UI, h Handler) int {
	patterns, refusal := NewPatterns(c.Patterns)
	if refusal != nil {
		return ui.Fail(refusal, false)
	}
	found, failure := h.Handle(ctx, Query{Patterns: patterns})
	if failure != nil {
		return ui.Fail(failure, false)
	}
	for _, f := range found {
		_, _ = fmt.Fprintln(ui.Stdout, f)
	}
	return 0
}
