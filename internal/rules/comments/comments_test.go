package comments_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/donvargaxdev/anti-slop/internal/rules/comments"
)

func TestCatchesEveryCommentNoToolReads(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), comments.Analyzer, "catch")
}

func TestLeavesTheCommentsAToolReads(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), comments.Analyzer, "leave")
}
