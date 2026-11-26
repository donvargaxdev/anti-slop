package command

import (
	"errors"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

// Classify maps checker failures to problems for the recursion command.
func Classify(err error) ([]*cli.Problem, bool) {
	var failure port.CheckFailure
	if !errors.As(err, &failure) {
		return nil, false
	}
	switch e := failure.(type) {
	case *port.Failed:
		return []*cli.Problem{cli.NewProblem(cli.CodeEnvironment, "recursion.check-failed",
			"the packages cannot be loaded: "+e.Reason,
			"Check the patterns and that the packages build, then run it again.")}, true
	case *port.Cancelled:
		return []*cli.Problem{cli.Interrupted()}, true
	}
	return []*cli.Problem{cli.Internal(failure)}, true
}
