package command

import (
	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

// Classify maps checker failures to problems for the conditions command.
func Classify(failure port.CheckFailure) *cli.Problem {
	switch e := failure.(type) {
	case *port.Failed:
		return cli.NewProblem(cli.CodeEnvironment, "conditions.check-failed",
			"the packages cannot be loaded: "+e.Reason,
			"Check the patterns and that the packages build, then run it again.")
	case *port.Cancelled:
		return cli.Interrupted()
	}
	return cli.Internal(failure)
}
