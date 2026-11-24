package command_test

import (
	"errors"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/tests/command"
)

type foreignFailure struct{ *port.Failed }

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"unrelated", errors.New("other"), 0},
		{"load failure", &port.Failed{Reason: "missing package"}, cli.CodeEnvironment},
		{"cancelled", &port.Cancelled{}, cli.Interrupted().Code},
		{"unknown check failure", foreignFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, ok := command.Classify(tc.err)
			if tc.code == 0 {
				if ok || len(problems) != 0 {
					t.Errorf("Classify(%v) = %v, %v, want no problems and false", tc.err, problems, ok)
				}
				return
			}
			if !ok || len(problems) != 1 || problems[0].Code != tc.code {
				t.Errorf("Classify(%v) = %v, %v, want one problem with exit %d and true", tc.err, problems, ok, tc.code)
			}
		})
	}
}
