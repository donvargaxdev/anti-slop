package tests_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/tests"
)

func TestReportsTestSmells(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), tests.Analyzer, "catch", "leave")
}
