package command_test

import (
	"errors"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/recursion/command"
)

type unexpectedFailure struct{ *port.Failed }

func TestClassifyCheckerFailures(t *testing.T) {
	for _, tc := range []struct {
		failure error
		code    int
		rule    string
	}{
		{&port.Failed{Reason: "invalid source"}, cli.CodeEnvironment, "recursion.check-failed"},
		{&port.Cancelled{}, cli.CodeInterrupted, "interrupted"},
		{unexpectedFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal, "internal"},
	} {
		problems, ok := command.Classify(tc.failure)
		if !ok || len(problems) != 1 || problems[0].Code != tc.code || problems[0].Rule != tc.rule {
			t.Errorf("Classify(%T) = %v, %v, want one problem with code %d and rule %q", tc.failure, problems, ok, tc.code, tc.rule)
		}
	}
}

func TestClassifyDeclinesUnrelatedErrors(t *testing.T) {
	if problems, ok := command.Classify(errors.New("unrelated")); ok || problems != nil {
		t.Errorf("Classify(unrelated) = %v, %v, want nil, false", problems, ok)
	}
}
