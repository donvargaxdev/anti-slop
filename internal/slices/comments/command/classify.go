package command

import (
	"errors"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

// Classify is the comments slice's classifier: it maps the checker port's
// sealed failures to problems, each with its code, rule and fix, and says
// whether err was one of them. Any other error is not this slice's, so the UI
// tries the next classifier and, with none left, reports it as the bug it is.
//
// Each kind below is one the command's scenarios and tests read: packages
// that cannot be loaded, the environment's, and a check an interrupt
// cancelled, exit 130 (decision 8).
func Classify(err error) ([]*cli.Problem, bool) {
	var failure port.CheckFailure
	if !errors.As(err, &failure) {
		return nil, false
	}
	switch e := failure.(type) {
	case *port.Failed:
		return []*cli.Problem{cli.NewProblem(cli.CodeEnvironment, "comments.check-failed",
			"the packages cannot be loaded: "+e.Reason,
			"Check the patterns and that the packages build, then run it again.")}, true
	case *port.Cancelled:
		return []*cli.Problem{cli.Interrupted()}, true
	}
	return []*cli.Problem{cli.Internal(failure)}, true
}
