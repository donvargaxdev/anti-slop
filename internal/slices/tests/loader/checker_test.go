package loader_test

import (
	"path/filepath"
	"testing"

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
