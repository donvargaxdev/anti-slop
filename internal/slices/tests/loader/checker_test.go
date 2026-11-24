package loader_test

import (
	"path/filepath"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
	"github.com/donvargaxdev/anti-slop/internal/slices/tests/loader"
)

func TestCheckerContract(t *testing.T) {
	porttest.CheckerContract(t, func() porttest.CheckerCase {
		return porttest.CheckerCase{
			Checker: loader.Checker{Dir: filepath.Join("..", "..", "..", "..")},
			Clean:   "./internal/rules/tests/testdata/src/leave/sleep",
			Dirty:   "./internal/rules/tests/testdata/src/catch/sleep",
			Broken:  "./internal/rules/tests/testdata/src/missing/...",
		}
	})
}
