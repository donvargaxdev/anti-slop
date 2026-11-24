package porttest

import (
	"context"
	"slices"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

// Reporter is the small test API the contract suite needs, so a *testing.T
// satisfies it and the suite imports no testing package.
type Reporter interface {
	Helper()
	Errorf(string, ...any)
}

// CheckerCase is what an implementation hands CheckerContract: the Checker
// under test and three patterns it answers for. Clean names code with no
// findings, Dirty code with at least two, and Broken
// names nothing that can be loaded.
type CheckerCase struct {
	Checker port.Checker
	Clean   string
	Dirty   string
	Broken  string
}

// CheckerContract holds every port.Checker to the same behavior: no findings
// for clean code, findings placed from 1 and ordered for dirty code, the
// Failed failure for a pattern that cannot be loaded, and what a done context
// does. The in-memory fake and the loader both run it unchanged, which is why
// the fake may stand in for the loader. open returns a fresh case for each
// call.
func CheckerContract(t Reporter, open func() CheckerCase) {
	t.Helper()
	checkClean(t, open())
	checkDirty(t, open())
	checkTogether(t, open())
	checkBroken(t, open())
	checkDoneContext(t, open())
}

func checkClean(t Reporter, c CheckerCase) {
	t.Helper()
	found, failure := c.Checker.Check(context.Background(), []string{c.Clean})
	if failure != nil || len(found) != 0 {
		t.Errorf("Check(clean) = %v, %v, want no findings and no failure", found, failure)
	}
}

func checkDirty(t Reporter, c CheckerCase) {
	t.Helper()
	found, failure := c.Checker.Check(context.Background(), []string{c.Dirty})
	if failure != nil || len(found) < 2 {
		t.Errorf("Check(dirty) = %v, %v, want at least two findings and no failure", found, failure)
		return
	}
	for _, f := range found {
		if f.Path == "" || f.Line < 1 || f.Col < 1 || f.Rule == "" || f.Reason == "" {
			t.Errorf("Check(dirty) gave %+v, want a path, a line and a column from 1, a rule and a reason", f)
		}
	}
	if !slices.Equal(found, port.Merge(nil, found)) {
		t.Errorf("Check(dirty) = %v, want findings in order of path, line and column, none twice", found)
	}
}

func checkTogether(t Reporter, c CheckerCase) {
	t.Helper()
	alone, _ := c.Checker.Check(context.Background(), []string{c.Dirty})
	twice, failure := c.Checker.Check(context.Background(), []string{c.Dirty, c.Clean, c.Dirty})
	if failure != nil || !slices.Equal(alone, twice) {
		t.Errorf("Check(dirty, clean, dirty) = %v, %v, want the findings of dirty once: %v", twice, failure, alone)
	}
}

func checkBroken(t Reporter, c CheckerCase) {
	t.Helper()
	found, failure := c.Checker.Check(context.Background(), []string{c.Clean, c.Broken})
	if _, ok := failure.(*port.Failed); !ok || found != nil {
		t.Errorf("Check(broken) = %v, %v, want *port.Failed and no findings", found, failure)
	}
}

func checkDoneContext(t Reporter, c CheckerCase) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	found, failure := c.Checker.Check(ctx, []string{c.Dirty})
	if _, ok := failure.(*port.Cancelled); !ok || found != nil {
		t.Errorf("Check(done context) = %v, %v, want *port.Cancelled and no findings", found, failure)
	}
}
