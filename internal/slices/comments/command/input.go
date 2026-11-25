package command

import "github.com/donvargaxdev/anti-slop/internal/cli"

// Patterns holds checked Go package arguments.
type Patterns = cli.PackagePatterns

func NewPatterns(args []string) (Patterns, error) {
	return cli.NewPackagePatterns("comments", args)
}
