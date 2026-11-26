package funclen_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/funclen"
)

func TestFunctionLength(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), funclen.Analyzer, "catch", "leave")
}
