package command

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargaxdev/anti-slop/internal/cli"
)

// query is the form go/packages reads as a request of its own, file=path or
// name=x, instead of a pattern for go list.
var query = regexp.MustCompile(`^[A-Za-z]+=`)

// Patterns is the packages argument, checked where it enters: what the
// command line names to check. Its field is unexported and NewPatterns the
// one constructor that checks it, so holding Patterns proves the arguments
// were checked; the Query, the Handler and the port beneath them trust it and
// check it no more. The zero Patterns was never checked, and nothing here
// makes one.
type Patterns struct{ list []string }

// NewPatterns checks args, the packages as the command line gave them, and
// refuses what go list would read as something else: none at all, an empty
// one, one holding a NUL byte, one that starts with a dash, which go list
// takes for a flag, and one of the form name=value, which the package loader
// takes for a query. A refusal is the problem comments.pattern-invalid, exit
// 80, naming the argument as given; nothing is repaired.
func NewPatterns(args []string) (Patterns, error) {
	if len(args) == 0 {
		return Patterns{}, invalid("", "no package is named")
	}
	for _, arg := range args {
		switch {
		case arg == "":
			return Patterns{}, invalid(arg, "a package argument is empty")
		case strings.ContainsRune(arg, 0):
			return Patterns{}, invalid(arg, fmt.Sprintf("the package argument %q holds a NUL byte", arg))
		case strings.HasPrefix(arg, "-"):
			return Patterns{}, invalid(arg, fmt.Sprintf("the package argument %q starts with a dash, as a flag does", arg))
		case query.MatchString(arg):
			return Patterns{}, invalid(arg, fmt.Sprintf("the package argument %q is a query, not a pattern", arg))
		}
	}
	return Patterns{list: slices.Clone(args)}, nil
}

// invalid is the problem of arg, an argument that names no package.
func invalid(arg, message string) error {
	return cli.NewProblem(cli.CodeUsage, "comments.pattern-invalid", message,
		"Give the packages to check, as go list reads them: ./... or a directory.").With("pattern", arg)
}

// List is the patterns as they were given, in order. The slice is a copy.
func (p Patterns) List() []string { return slices.Clone(p.list) }
