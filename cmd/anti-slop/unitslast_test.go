package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
)

func unitslastClassifier(t *testing.T) cli.Classifier {
	t.Helper()
	field := reflect.ValueOf(commandLine{}).FieldByName("Unitslast")
	if !field.IsValid() {
		t.Fatal("the command line has no unitslast command")
	}
	command, ok := field.Interface().(interface {
		Classify(error) ([]*cli.Problem, bool)
	})
	if !ok {
		t.Fatal("the unitslast command has no failure classifier")
	}
	return command.Classify
}

func TestClassifyDeclinesUnrelatedErrors(t *testing.T) {
	classify := unitslastClassifier(t)
	if problems, ok := classify(errors.New("unrelated")); ok || problems != nil {
		t.Errorf("Classify(unrelated) = %v, %v, want nil, false", problems, ok)
	}
}

// Embedding keeps the port's failure interface while introducing an unknown type.
type unexpectedFailure struct{ *port.Failed }

func TestClassifyCheckerFailures(t *testing.T) {
	classify := unitslastClassifier(t)
	for _, tc := range []struct {
		name    string
		failure port.CheckFailure
		code    int
		rule    string
		message string
	}{
		{"load error", &port.Failed{Reason: "invalid source"}, cli.CodeEnvironment,
			"unitslast.check-failed", "the packages cannot be loaded: invalid source"},
		{"cancelled", &port.Cancelled{}, cli.CodeInterrupted,
			"interrupted", "interrupted before it finished"},
		{"unexpected failure", unexpectedFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal,
			"internal", "cannot check: unexpected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, ok := classify(tc.failure)
			if !ok || len(problems) != 1 {
				t.Fatalf("Classify(%T) = %v, %v, want one problem and true", tc.failure, problems, ok)
			}
			got := problems[0]
			if got.Code != tc.code || got.Rule != tc.rule || got.Message != tc.message {
				t.Errorf("problem = %+v, want code %d, rule %q and message %q", got, tc.code, tc.rule, tc.message)
			}
		})
	}
}

func unitslastChecker(t *testing.T) port.Checker {
	t.Helper()
	field := reflect.ValueOf(diskAdapters()).FieldByName("Unitslast")
	if !field.IsValid() {
		t.Fatal("the disk adapters have no unitslast checker")
	}
	checker, ok := field.Interface().(port.Checker)
	if !ok {
		t.Fatal("the unitslast disk adapter does not implement the checker port")
	}
	return checker
}

const unitslastFixtures = "../../internal/rules/unitslast/testdata/src"

func TestCheckerContract(t *testing.T) {
	checker := unitslastChecker(t)
	t.Chdir(unitslastFixtures)
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: checker,
			Clean:   "./leave",
			Dirty:   "./catch",
			Broken:  "./missing",
		}
	})
}

func TestCheckerRejectsAnEmptyPackageMatch(t *testing.T) {
	checker := unitslastChecker(t)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("go.mod", []byte("module example.test/empty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, failure := checker.Check(t.Context(), []string{"./..."})
	failed, ok := failure.(*port.Failed)
	if found != nil || !ok || failed.Reason != "the patterns match no package" {
		t.Errorf("Check(./...) = %v, %v, want no findings and a no-match failure", found, failure)
	}
}

func TestCheckerKeepsAbsolutePathForFindingsOutsideItsDirectory(t *testing.T) {
	checker := unitslastChecker(t)
	name, err := filepath.Abs(filepath.Join(unitslastFixtures, "catch", "catch.go"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(unitslastFixtures, "leave"))
	found, failure := checker.Check(t.Context(), []string{name})
	if failure != nil || len(found) == 0 {
		t.Fatalf("Check(%q) = %v, %v, want findings and no failure", name, found, failure)
	}
	for _, finding := range found {
		if finding.Path != name {
			t.Errorf("finding path = %q, want %q", finding.Path, name)
		}
	}
}
