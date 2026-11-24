package command

import (
	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

type Patterns = port.Patterns

func NewPatterns(args []string) (Patterns, error) {
	patterns, refusal := port.NewPatterns(args)
	if refusal != nil {
		return Patterns{}, cli.NewProblem(cli.CodeUsage, "comments.pattern-invalid", refusal.Reason,
			"Give the packages to check, as go list reads them: ./... or a directory.").With("pattern", refusal.Pattern)
	}
	return patterns, nil
}
