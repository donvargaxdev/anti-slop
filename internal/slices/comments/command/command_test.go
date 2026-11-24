package command_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/command"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port/porttest"
)

func ui() (*cli.UI, *bytes.Buffer, *bytes.Buffer) {
	var out, errs bytes.Buffer
	return &cli.UI{Stdout: &out, Stderr: &errs, Classifiers: []cli.Classifier{command.Classify}}, &out, &errs
}

func TestRunPrintsEachFindingAsALineAndExitsZero(t *testing.T) {
	u, out, errs := ui()
	h := command.Handler{Checker: porttest.Checker{Findings: map[string][]port.Finding{"./p": {
		{Path: "p/a.go", Line: 3, Col: 1, Rule: "comments", Reason: "first"},
		{Path: "p/a.go", Line: 9, Col: 4, Rule: "comments", Reason: "second"},
	}}}}
	code := command.CLI{Patterns: []string{"./p"}}.Run(t.Context(), u, h)
	want := "p/a.go:3:1: comments: first\np/a.go:9:4: comments: second\n"
	if code != 0 || out.String() != want || errs.Len() != 0 {
		t.Errorf("Run = %d, stdout %q, stderr %q, want 0, %q and nothing", code, out.String(), errs.String(), want)
	}
}

func TestRunPrintsNothingWhenThereIsNoFinding(t *testing.T) {
	u, out, errs := ui()
	code := command.CLI{Patterns: []string{"./p"}}.Run(t.Context(), u, command.Handler{Checker: porttest.Checker{}})
	if code != 0 || out.Len() != 0 || errs.Len() != 0 {
		t.Errorf("Run = %d, stdout %q, stderr %q, want 0 and nothing", code, out.String(), errs.String())
	}
}

func TestRunReportsAPackageThatCannotBeLoadedAsExit82(t *testing.T) {
	u, out, errs := ui()
	h := command.Handler{Checker: porttest.Checker{Failures: map[string]port.CheckFailure{"./p": &port.Failed{Reason: "boom"}}}}
	code := command.CLI{Patterns: []string{"./p"}}.Run(t.Context(), u, h)
	if code != 82 || out.Len() != 0 || !strings.Contains(errs.String(), "anti-slop: the packages cannot be loaded: boom") {
		t.Errorf("Run = %d, stdout %q, stderr %q, want 82, nothing and the reason", code, out.String(), errs.String())
	}
}

func TestRunReportsACancelledCheckAsExit130(t *testing.T) {
	u, _, _ := ui()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := command.CLI{Patterns: []string{"./p"}}.Run(ctx, u, command.Handler{Checker: porttest.Checker{}})
	if code != 130 {
		t.Errorf("Run = %d, want 130", code)
	}
}

func TestRunRefusesABadPatternWithExit80BeforeAnyCheck(t *testing.T) {
	for _, args := range [][]string{nil, {""}, {"./p", ""}, {"-x"}, {"./p\x00"}, {"file=x.go"}, {"name=p"}} {
		u, out, errs := ui()
		h := command.Handler{Checker: porttest.Checker{Failures: map[string]port.CheckFailure{"./p": &port.Failed{}}}}
		code := command.CLI{Patterns: args}.Run(t.Context(), u, h)
		if code != 80 || out.Len() != 0 || !strings.Contains(errs.String(), "anti-slop:") {
			t.Errorf("Run(%q) = %d, stdout %q, stderr %q, want 80, nothing and a message", args, code, out.String(), errs.String())
		}
	}
}

func TestPatternsAreAcceptedAsGoListReadsThem(t *testing.T) {
	args := []string{"./...", "./internal/rules/comments/testdata/src/catch/...", "example.com/m/p", "./x=y", "."}
	got, err := command.NewPatterns(args)
	if err != nil || strings.Join(got.List(), " ") != strings.Join(args, " ") {
		t.Errorf("NewPatterns = %v, %v, want them all", got.List(), err)
	}
	got.List()[0] = "changed"
	if got.List()[0] != "./..." {
		t.Error("List gives the patterns themselves, not a copy")
	}
	args[1] = "changed"
	if got.List()[1] == "changed" {
		t.Error("NewPatterns keeps the caller's slice, not a copy")
	}
}

func TestARefusalNamesTheArgumentAsGiven(t *testing.T) {
	_, err := command.NewPatterns([]string{"-x"})
	var p *cli.Problem
	if !errors.As(err, &p) || p.Rule != "comments.pattern-invalid" || p.Code != cli.CodeUsage {
		t.Fatalf("NewPatterns(-x) = %v, want the problem comments.pattern-invalid", err)
	}
}

func TestClassifyLeavesAnotherSlicesErrorsAlone(t *testing.T) {
	if problems, ok := command.Classify(errors.New("other")); ok || problems != nil {
		t.Errorf("Classify(other) = %v, %v, want none and false", problems, ok)
	}
}

func TestHelpGivesEveryExitCodeTheCommandReturns(t *testing.T) {
	help := command.CLI{}.Help()
	for _, code := range []string{"0  ", "80  ", "82  ", "100  ", "130  "} {
		if !strings.Contains(help, "      "+code) {
			t.Errorf("Help lacks exit code %q", code)
		}
	}
}
