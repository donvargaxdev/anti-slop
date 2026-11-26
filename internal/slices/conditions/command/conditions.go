// Package command implements the conditions subcommand.
package command

import (
	"context"
	"fmt"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

type CLI struct {
	Patterns []string `arg:"" help:"The packages to check, as go list reads them."`
}

func (CLI) Help() string {
	return "\nReport if and for conditions with more than two logical operators.\n\n" +
		"    Stdout format: path:line:col: conditions: <reason>\n" +
		"    Exit codes:\n" +
		"      0  success, with or without findings\n" +
		"      80  the command line is wrong\n" +
		"      82  the packages cannot be loaded\n" +
		"      100  an unclassified error\n" +
		"      130  interrupted\n"
}

type Query struct {
	Patterns cli.PackagePatterns
}

type Handler struct {
	Checker port.Checker
}

func (h Handler) Handle(ctx context.Context, q Query) ([]port.Finding, port.CheckFailure) {
	return h.Checker.Check(ctx, q.Patterns.List())
}

func (c CLI) Run(ctx context.Context, ui *cli.UI, h Handler) int {
	patterns, err := cli.NewPackagePatterns("conditions", c.Patterns)
	if err != nil {
		return ui.Fail(err, false)
	}
	found, failure := h.Handle(ctx, Query{Patterns: patterns})
	if failure != nil {
		return ui.Emit(false, []*cli.Problem{Classify(failure)})
	}
	for _, f := range found {
		_, _ = fmt.Fprintln(ui.Stdout, f)
	}
	return 0
}
