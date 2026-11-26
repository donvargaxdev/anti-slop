package unitslast_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/unitslast"
)

func TestReportsLeadingUnitsAndQualifiers(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), unitslast.Analyzer, "catch", "leave")
}
