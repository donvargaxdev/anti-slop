package port_test

import (
	"slices"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/slices/comments/port"
)

func TestFindingIsALineOfOutput(t *testing.T) {
	f := port.Finding{Path: "a/b.go", Line: 3, Col: 7, Rule: "comments", Reason: "no tool reads it"}
	if got, want := f.String(), "a/b.go:3:7: comments: no tool reads it"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestMergeOrdersByPathThenLineThenColumnAndDropsRepeats(t *testing.T) {
	at := func(path string, line, col int) port.Finding {
		return port.Finding{Path: path, Line: line, Col: col, Rule: "r", Reason: "x"}
	}
	got := port.Merge(
		[]port.Finding{at("b.go", 1, 1), at("a.go", 2, 5), at("a.go", 2, 1)},
		[]port.Finding{at("a.go", 10, 1), at("a.go", 2, 5), at("a.go", 1, 9)},
	)
	want := []port.Finding{at("a.go", 1, 9), at("a.go", 2, 1), at("a.go", 2, 5), at("a.go", 10, 1), at("b.go", 1, 1)}
	if !slices.Equal(got, want) {
		t.Errorf("Merge = %v, want %v", got, want)
	}
}

func TestMergeKeepsTwoReasonsAtOnePlace(t *testing.T) {
	a := port.Finding{Path: "a.go", Line: 1, Col: 1, Rule: "r", Reason: "one"}
	b := port.Finding{Path: "a.go", Line: 1, Col: 1, Rule: "r", Reason: "two"}
	if got := port.Merge([]port.Finding{b}, []port.Finding{a}); !slices.Equal(got, []port.Finding{a, b}) {
		t.Errorf("Merge = %v, want %v then %v", got, a, b)
	}
}

func TestFailuresSayWhatHappened(t *testing.T) {
	if got := (&port.Failed{Reason: "no packages"}).Error(); got != "cannot check: no packages" {
		t.Errorf("Failed.Error() = %q", got)
	}
	if got := (&port.Cancelled{}).Error(); got != "cancelled check" {
		t.Errorf("Cancelled.Error() = %q", got)
	}
}
