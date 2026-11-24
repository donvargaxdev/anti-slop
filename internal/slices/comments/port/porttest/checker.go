// Package porttest provides the in-memory fake of the comments slice's port.
package porttest

import (
	"context"

	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
)

// Checker answers each pattern with the findings or the failure a test
// configured, in memory.
type Checker struct {
	Findings map[string][]port.Finding
	Failures map[string]port.CheckFailure
}

var _ port.Checker = Checker{}

// Check returns the failure configured for the first pattern that has one, or
// the findings of every pattern in turn, in order of path, line and column
// with none twice. A done ctx is the Cancelled failure.
func (c Checker) Check(ctx context.Context, patterns []string) ([]port.Finding, port.CheckFailure) {
	if ctx.Err() != nil {
		return nil, &port.Cancelled{}
	}
	for _, pattern := range patterns {
		if failure, ok := c.Failures[pattern]; ok {
			return nil, failure
		}
	}
	var found []port.Finding
	for _, pattern := range patterns {
		found = port.Merge(found, c.Findings[pattern])
	}
	return found, nil
}
