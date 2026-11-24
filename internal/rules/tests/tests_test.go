package tests_test

import (
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/tests"
)

func TestAnalyzers(t *testing.T) {
	for _, tc := range []struct {
		name string
		rule *analysis.Analyzer
	}{
		{"sleep", tests.Sleep},
		{"polling", tests.Polling},
		{"environment", tests.Environment},
		{"flakes", tests.Flakes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			analysistest.Run(t, analysistest.TestData(), tc.rule, "catch/"+tc.name, "leave/"+tc.name)
		})
	}
}
