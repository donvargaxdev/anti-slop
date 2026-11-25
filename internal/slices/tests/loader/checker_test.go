package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
	"github.com/donvargaxdev/anti-slop/internal/slices/tests/loader"
)

func TestCheckerContract(t *testing.T) {
	dir, err := filepath.Abs("../../../../internal/rules/tests/testdata/src")
	if err != nil {
		t.Fatal(err)
	}
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: loader.Checker{Dir: dir},
			Clean:   "./leave",
			Dirty:   "./catch",
			Broken:  "./missing",
		}
	})
}

func TestCheckerRejectsAnEmptyPackageMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/empty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	checker := loader.Checker{Dir: dir}
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
	checker := loader.Checker{Dir: filepath.Join(dir, "leave")}
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
