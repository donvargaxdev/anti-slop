package porttest_test

import (
	"context"
	"slices"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port/porttest"
)

func TestTheFakeHoldsTheContract(t *testing.T) {
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: porttest.Checker{
				Findings: map[string][]port.Finding{"dirty": {
					{Path: "a.go", Line: 1, Col: 1, Rule: "comments", Reason: "one"},
					{Path: "a.go", Line: 3, Col: 2, Rule: "comments", Reason: "two"},
				}},
				Failures: map[string]port.CheckFailure{"broken": &port.Failed{Reason: "no such package"}},
			},
			Clean: "clean", Dirty: "dirty", Broken: "broken",
		}
	})
}

type recorder struct{ errors []string }

func (*recorder) Helper() {}
func (r *recorder) Errorf(format string, _ ...any) {
	r.errors = append(r.errors, format)
}

func TestTheContractRefusesAFakeThatBreaksIt(t *testing.T) {
	good := func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: porttest.Checker{
				Findings: map[string][]port.Finding{"dirty": {
					{Path: "a.go", Line: 1, Col: 1, Rule: "comments", Reason: "one"},
					{Path: "a.go", Line: 3, Col: 2, Rule: "comments", Reason: "two"},
				}},
				Failures: map[string]port.CheckFailure{"broken": &port.Failed{Reason: "no such package"}},
			},
			Clean: "clean", Dirty: "dirty", Broken: "broken",
		}
	}
	for name, mutate := range map[string]func(*porttest.CheckerCase){
		"a clean pattern with findings": func(c *porttest.CheckerCase) { c.Clean = "dirty" },
		"a dirty pattern with none":     func(c *porttest.CheckerCase) { c.Dirty = "clean" },
		"a broken pattern that loads":   func(c *porttest.CheckerCase) { c.Broken = "clean" },
		"a finding with no place": func(c *porttest.CheckerCase) {
			c.Checker = porttest.Checker{Findings: map[string][]port.Finding{"dirty": {
				{Path: "a.go", Line: 0, Col: 1, Rule: "comments", Reason: "one"},
				{Path: "a.go", Line: 1, Col: 1, Rule: "comments", Reason: "two"},
			}}, Failures: map[string]port.CheckFailure{"broken": &port.Failed{}}}
		},
		"a cancelled failure for a broken pattern": func(c *porttest.CheckerCase) {
			c.Checker = porttest.Checker{Findings: c.Checker.(porttest.Checker).Findings,
				Failures: map[string]port.CheckFailure{"broken": &port.Cancelled{}}}
		},
	} {
		c := good()
		mutate(&c)
		r := &recorder{}
		porttest.CheckerContract(r, func() porttest.CheckerCase { return c })
		if len(r.errors) == 0 {
			t.Errorf("%s: the contract found nothing wrong", name)
		}
	}
}

func TestTheFakeRepeatsNoFindingAndOrdersThem(t *testing.T) {
	a := port.Finding{Path: "a.go", Line: 2, Col: 1, Rule: "comments", Reason: "x"}
	b := port.Finding{Path: "a.go", Line: 1, Col: 1, Rule: "comments", Reason: "x"}
	fake := porttest.Checker{Findings: map[string][]port.Finding{"p": {a}, "q": {a, b}}}
	got, failure := fake.Check(t.Context(), []string{"p", "q"})
	if failure != nil || len(got) != 2 || got[0] != b || got[1] != a {
		t.Errorf("Check(p, q) = %v, %v, want %v then %v", got, failure, b, a)
	}
}

type deviant struct {
	port.Checker
	reverse, repeat, ignoreContext bool
}

func (d deviant) Check(ctx context.Context, patterns []string) ([]port.Finding, port.CheckFailure) {
	if d.ignoreContext {
		ctx = context.Background()
	}
	found, failure := d.Checker.Check(ctx, patterns)
	if d.reverse {
		slices.Reverse(found)
	}
	if d.repeat && len(patterns) > 1 {
		found = append(found, found...)
	}
	return found, failure
}

func TestTheContractRefusesACheckerThatBreaksIt(t *testing.T) {
	fake := porttest.Checker{
		Findings: map[string][]port.Finding{"dirty": {
			{Path: "a.go", Line: 1, Col: 1, Rule: "comments", Reason: "one"},
			{Path: "a.go", Line: 3, Col: 2, Rule: "comments", Reason: "two"},
		}},
		Failures: map[string]port.CheckFailure{"broken": &port.Failed{Reason: "no such package"}},
	}
	for name, checker := range map[string]port.Checker{
		"findings out of order":          deviant{Checker: fake, reverse: true},
		"findings repeated for a repeat": deviant{Checker: fake, repeat: true},
		"a done context ignored":         deviant{Checker: fake, ignoreContext: true},
	} {
		r := &recorder{}
		porttest.CheckerContract(r, func() porttest.CheckerCase {
			return porttest.CheckerCase{Checker: checker, Clean: "clean", Dirty: "dirty", Broken: "broken"}
		})
		if len(r.errors) == 0 {
			t.Errorf("%s: the contract found nothing wrong", name)
		}
	}
}
