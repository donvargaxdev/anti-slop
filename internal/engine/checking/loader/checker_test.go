package loader_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/loader"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
	"github.com/donvargaxdev/anti-slop/internal/rules/recursion"
	"github.com/donvargaxdev/anti-slop/internal/rules/tests"
)

func TestCheckerContract(t *testing.T) {
	for _, rule := range []*analysis.Analyzer{tests.Analyzer, recursion.Analyzer} {
		t.Run(rule.Name, func(t *testing.T) {
			dir := filepath.Join("..", "..", "..", "rules", rule.Name, "testdata", "src")
			porttest.CheckerContract(t, func() porttest.CheckerCase {
				return porttest.CheckerCase{
					Checker: loader.Checker{Dir: dir, Analyzer: rule},
					Clean:   "./leave",
					Dirty:   "./catch",
					Broken:  "./missing",
				}
			})
		})
	}
}

func TestCheckerDiscardsFindingsOnAnalyzerFailure(t *testing.T) {
	rule := &analysis.Analyzer{
		Run: func(pass *analysis.Pass) (any, error) {
			pass.Reportf(pass.Files[0].Pos(), "partial finding")
			return nil, errors.New("analysis failed")
		},
	}
	dir := filepath.Join("..", "..", "..", "rules", "recursion", "testdata", "src")
	found, failure := (loader.Checker{Dir: dir, Analyzer: rule}).Check(t.Context(), []string{"./catch"})
	failed, ok := failure.(*port.Failed)
	if found != nil || !ok || failed.Reason != "analysis failed" {
		t.Errorf("Check returned %v, %v, want no findings and an analysis failure", found, failure)
	}
}

func TestCheckerRejectsAnEmptyPackageMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/empty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	checker := loader.Checker{Dir: dir, Analyzer: tests.Analyzer}
	found, failure := checker.Check(t.Context(), []string{"./..."})
	failed, ok := failure.(*port.Failed)
	if found != nil || !ok || failed.Reason != "the patterns match no package" {
		t.Errorf("Check(./...) = %v, %v, want no findings and a no-match failure", found, failure)
	}
}

func TestCheckerKeepsAbsolutePathForFindingsOutsideItsDirectory(t *testing.T) {
	dir, err := filepath.Abs("../../../../internal/rules/tests/testdata/src")
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "catch", "catch_test.go")
	checker := loader.Checker{Dir: filepath.Join(dir, "leave"), Analyzer: tests.Analyzer}
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
