package command_test

import (
	"errors"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/unitslast/command"
)

func TestClassifyDeclinesUnrelatedErrors(t *testing.T) {
	if problems, ok := command.Classify(errors.New("unrelated")); ok || problems != nil {
		t.Errorf("Classify(unrelated) = %v, %v, want nil, false", problems, ok)
	}
}

// Embedding keeps the port's failure interface while introducing an unknown type.
type unexpectedFailure struct{ *port.Failed }

func TestClassifyCheckerFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure port.CheckFailure
		code    int
		rule    string
		message string
	}{
		{"load error", &port.Failed{Reason: "invalid source"}, cli.CodeEnvironment,
			"unitslast.check-failed", "the packages cannot be loaded: invalid source"},
		{"cancelled", &port.Cancelled{}, cli.CodeInterrupted,
			"interrupted", "interrupted before it finished"},
		{"unexpected failure", unexpectedFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal,
			"internal", "cannot check: unexpected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, ok := command.Classify(tc.failure)
			if !ok || len(problems) != 1 {
				t.Fatalf("Classify(%T) = %v, %v, want one problem and true", tc.failure, problems, ok)
			}
			got := problems[0]
			if got.Code != tc.code || got.Rule != tc.rule || got.Message != tc.message {
				t.Errorf("problem = %+v, want code %d, rule %q and message %q", got, tc.code, tc.rule, tc.message)
			}
		})
	}
}
