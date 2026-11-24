package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

type CLI struct {
	Patterns []string `arg:"" optional:"" help:"One or more packages to check, as go list reads them: ./..., a directory or an import path."`
}

func (CLI) Help() string {
	return "\nReport sleeps, wall-clock polling, process environment changes and flake masking in Go tests.\n\n" +
		"    Each finding is one line on stdout: path:line:col: tests: <reason>.\n" +
		"    There is no --json output.\n" +
		"    Exit codes:\n" +
		"      0  success, with or without findings\n" +
		"      80  the command line is wrong\n" +
		"      82  the packages cannot be loaded\n" +
		"      100  an error nothing classified, a bug\n" +
		"      130  interrupted\n"
}

func (c CLI) Run(ctx context.Context, ui *cli.UI, checker port.Checker) int {
	patterns, refusal := port.NewPatterns(c.Patterns)
	if refusal != nil {
		return ui.Fail(cli.NewProblem(cli.CodeUsage, "tests.pattern-invalid", refusal.Reason,
			"Give the packages to check, as go list reads them: ./... or a directory.").With("pattern", refusal.Pattern), false)
	}
	found, failure := checker.Check(ctx, patterns.List())
	if failure != nil {
		return ui.Fail(failure, false)
	}
	for _, finding := range found {
		_, _ = fmt.Fprintln(ui.Stdout, finding)
	}
	return 0
}

func Classify(err error) ([]*cli.Problem, bool) {
	var failure port.CheckFailure
	if !errors.As(err, &failure) {
		return nil, false
	}
	switch failure := failure.(type) {
	case *port.Failed:
		return []*cli.Problem{cli.NewProblem(cli.CodeEnvironment, "tests.check-failed",
			"the packages cannot be loaded: "+failure.Reason,
			"Check the patterns and that the packages build, then run it again.")}, true
	case *port.Cancelled:
		return []*cli.Problem{cli.Interrupted()}, true
	}
	return []*cli.Problem{cli.Internal(failure)}, true
}
