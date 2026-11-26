package recursion_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/recursion"
)

func TestReportsRecursion(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), recursion.Analyzer, "catch", "leave")
}
