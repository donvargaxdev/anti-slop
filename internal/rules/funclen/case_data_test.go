package funclen

import (
	"go/ast"
	"go/token"
	"testing"
)

// A synthetic span puts the exclusive AST end at a line start, which an
// ordinary parsed element followed by a comma cannot exercise.
func TestCaseDataStopsBeforeExclusiveEnd(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("table_test.go", -1, 20)
	file.SetLines([]int{0, 5, 10, 15})
	table := &ast.CompositeLit{
		Lbrace: file.Pos(0),
		Elts: []ast.Expr{
			&ast.BasicLit{ValuePos: file.Pos(5), ValueEnd: file.Pos(10), Kind: token.STRING},
		},
		Rbrace: file.Pos(15),
	}

	lines := make(map[int]struct{})
	markCaseData(fset, lines, table)

	if _, marked := lines[2]; !marked || len(lines) != 1 {
		t.Errorf("case data lines = %v, want only line 2", lines)
	}
}
