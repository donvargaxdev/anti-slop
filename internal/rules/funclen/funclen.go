// Package funclen reports Go function bodies longer than 70 physical lines.
package funclen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	name  = "funclen"
	limit = 70
)

// Analyzer checks function, method and function literal bodies.
var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "report function bodies longer than 70 lines",
	Run: func(pass *analysis.Pass) (any, error) {
		reportFiles(pass.Fset, pass.Files, pass.Report)
		return nil, nil
	},
}

// reportFiles checks physical lines so line directives cannot change the count.
func reportFiles(fset *token.FileSet, files []*ast.File, report func(analysis.Diagnostic)) {
	for _, file := range files {
		if ast.IsGenerated(file) {
			continue
		}
		var tablesUsed tables
		if strings.HasSuffix(fset.File(file.Pos()).Name(), "_test.go") {
			tablesUsed = caseTables(file)
		}
		for node := range ast.Preorder(file) {
			body := functionBody(node)
			if body == nil {
				continue
			}
			lines := interiorLines(fset, body.Lbrace, body.Rbrace)
			if lines > limit {
				lines -= testCaseLines(fset, body, tablesUsed)
			}
			if lines > limit {
				report(analysis.Diagnostic{
					Pos: node.Pos(), Category: name,
					Message: fmt.Sprintf("function body is %d lines; limit is %d", lines, limit),
				})
			}
		}
	}
}

func interiorLines(fset *token.FileSet, open, close token.Pos) int {
	return max(0, fset.PositionFor(close, false).Line-fset.PositionFor(open, false).Line-1)
}

func functionBody(node ast.Node) *ast.BlockStmt {
	switch function := node.(type) {
	case *ast.FuncDecl:
		return function.Body
	case *ast.FuncLit:
		return function.Body
	}
	return nil
}

func testCaseLines(fset *token.FileSet, body *ast.BlockStmt, tablesUsed tables) int {
	lines := make(map[int]struct{})
	for table := range tablesUsed {
		if body.Lbrace < table.Pos() && table.End() <= body.Rbrace {
			markCaseData(fset, lines, table)
		}
	}
	return len(lines)
}

func markCaseData(fset *token.FileSet, lines map[int]struct{}, table *ast.CompositeLit) {
	firstInterior := fset.PositionFor(table.Lbrace, false).Line + 1
	lastInterior := fset.PositionFor(table.Rbrace, false).Line - 1
	for _, element := range table.Elts {
		first := max(firstInterior, fset.PositionFor(element.Pos(), false).Line)
		last := min(lastInterior, fset.PositionFor(element.End()-1, false).Line)
		for line := first; line <= last; line++ {
			lines[line] = struct{}{}
		}
	}
}
