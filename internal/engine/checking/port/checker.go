// Package port defines the checker for commands that report findings in Go packages.
package port

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Finding is one diagnostic a rule reports, at the place it starts. Line and
// Col count from 1.
type Finding struct {
	Path   string
	Line   int
	Col    int
	Rule   string
	Reason string
}

// String is the finding as a line of output: path:line:col: rule: reason.
func (f Finding) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", f.Path, f.Line, f.Col, f.Rule, f.Reason)
}

// CheckFailure describes why a check could not be made, without exposing the
// package loader's own errors to the command.
//
//sumtype:decl
type CheckFailure interface {
	error
	checkFailure()
}

// Failed is returned when the packages a pattern names cannot be loaded: it
// matches none, or one of the packages contains invalid source.
type Failed struct{ Reason string }

func (*Failed) checkFailure()   {}
func (e *Failed) Error() string { return "cannot check: " + e.Reason }

// Cancelled is returned when the check's context was done before the check
// ended, as an interrupt makes it: the findings are not all there.
type Cancelled struct{}

func (*Cancelled) checkFailure()   {}
func (e *Cancelled) Error() string { return "cancelled check" }

// Checker reports findings in the packages patterns name, as
// go list reads patterns. Its findings are in order of path, line and column,
// and none is given twice. ctx carries only the check's cancellation and
// deadline, never a value.
type Checker interface {
	Check(ctx context.Context, patterns []string) ([]Finding, CheckFailure)
}

// Merge is the findings of a and b together, in order of path, line and
// column, each once. It is the order and the uniqueness every Checker keeps.
func Merge(a, b []Finding) []Finding {
	all := slices.Concat(a, b)
	slices.SortFunc(all, func(x, y Finding) int {
		if order := strings.Compare(x.Path, y.Path); order != 0 {
			return order
		}
		if order := x.Line - y.Line; order != 0 {
			return order
		}
		if order := x.Col - y.Col; order != 0 {
			return order
		}
		return strings.Compare(x.Reason, y.Reason)
	})
	return slices.Compact(all)
}
