package cli

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// query is the form go/packages reads as a request of its own, file=path or
// name=x, instead of a pattern for go list.
var query = regexp.MustCompile(`^[A-Za-z]+=`)

// PackagePatterns holds arguments validated as Go package patterns.
type PackagePatterns struct{ list []string }

// NewPackagePatterns checks args, the packages as the command line gave them, and
// refuses what go list would read as something else: none at all, an empty
// one, one holding a NUL byte, one that starts with a dash, which go list
// takes for a flag, and one of the form name=value, which the package loader
// takes for a query. A refusal is a usage problem naming the argument as given.
func NewPackagePatterns(command string, args []string) (PackagePatterns, error) {
	if len(args) == 0 {
		return PackagePatterns{}, invalidPackagePattern(command, "", "no package is named")
	}
	for _, arg := range args {
		switch {
		case arg == "":
			return PackagePatterns{}, invalidPackagePattern(command, arg, "a package argument is empty")
		case strings.ContainsRune(arg, 0):
			return PackagePatterns{}, invalidPackagePattern(command, arg, fmt.Sprintf("the package argument %q holds a NUL byte", arg))
		case strings.HasPrefix(arg, "-"):
			return PackagePatterns{}, invalidPackagePattern(command, arg, fmt.Sprintf("the package argument %q starts with a dash, as a flag does", arg))
		case query.MatchString(arg):
			return PackagePatterns{}, invalidPackagePattern(command, arg, fmt.Sprintf("the package argument %q is a query, not a pattern", arg))
		}
	}
	return PackagePatterns{list: slices.Clone(args)}, nil
}

func invalidPackagePattern(command, arg, message string) error {
	return NewProblem(CodeUsage, command+".pattern-invalid", message,
		"Give the packages to check, as go list reads them: ./... or a directory.").With("pattern", arg)
}

// List is the patterns as they were given, in order. The slice is a copy.
func (p PackagePatterns) List() []string { return slices.Clone(p.list) }
