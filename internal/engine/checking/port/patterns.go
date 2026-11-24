package port

import (
	"fmt"
	"slices"
	"strings"
)

type Patterns struct{ list []string }

type InvalidPattern struct {
	Pattern string
	Reason  string
}

func NewPatterns(args []string) (Patterns, *InvalidPattern) {
	if len(args) == 0 {
		return Patterns{}, &InvalidPattern{Reason: "no package is named"}
	}
	for _, arg := range args {
		switch {
		case arg == "":
			return Patterns{}, &InvalidPattern{Reason: "a package argument is empty"}
		case strings.ContainsRune(arg, 0):
			return Patterns{}, &InvalidPattern{arg, fmt.Sprintf("the package argument %q holds a NUL byte", arg)}
		case strings.HasPrefix(arg, "-"):
			return Patterns{}, &InvalidPattern{arg, fmt.Sprintf("the package argument %q starts with a dash, as a flag does", arg)}
		case loaderQuery(arg):
			return Patterns{}, &InvalidPattern{arg, fmt.Sprintf("the package argument %q is a query, not a pattern", arg)}
		}
	}
	return Patterns{list: slices.Clone(args)}, nil
}

func (p Patterns) List() []string { return slices.Clone(p.list) }

func loaderQuery(pattern string) bool {
	key, _, ok := strings.Cut(pattern, "=")
	if !ok || key == "" {
		return false
	}
	for _, r := range key {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}
