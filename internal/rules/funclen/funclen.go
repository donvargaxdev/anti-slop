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
		testFile := strings.HasSuffix(fset.File(file.Pos()).Name(), "_test.go")
		for node := range ast.Preorder(file) {
			var body *ast.BlockStmt
			switch function := node.(type) {
			case *ast.FuncDecl:
				body = function.Body
			case *ast.FuncLit:
				body = function.Body
			}
			if body == nil {
				continue
			}
			lines := interiorLines(fset, body.Lbrace, body.Rbrace)
			if testFile && lines > limit {
				lines -= testCaseLines(fset, body)
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

func testCaseLines(fset *token.FileSet, body *ast.BlockStmt) int {
	lines := make(map[int]struct{})
	for node := range ast.Preorder(body) {
		switch node := node.(type) {
		case *ast.RangeStmt:
			if table, ok := ast.Unparen(node.X).(*ast.CompositeLit); ok {
				markCaseData(fset, lines, table)
			}
		case *ast.BlockStmt:
			markAdjacentTables(fset, lines, node.List)
		case *ast.CaseClause:
			markAdjacentTables(fset, lines, node.Body)
		case *ast.CommClause:
			markAdjacentTables(fset, lines, node.Body)
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

func markAdjacentTables(fset *token.FileSet, lines map[int]struct{}, statements []ast.Stmt) {
	for index := 1; index < len(statements); index++ {
		loop, ok := statements[index].(*ast.RangeStmt)
		if !ok {
			continue
		}
		if table := adjacentCaseTable(statements[index-1], loop.X); table != nil {
			markCaseData(fset, lines, table)
		}
	}
}

func adjacentCaseTable(previous ast.Stmt, expr ast.Expr) *ast.CompositeLit {
	declaration, ok := previous.(*ast.AssignStmt)
	if !ok || declaration.Tok != token.DEFINE {
		return nil
	}
	if len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 {
		return nil
	}
	identifier, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	local, ok := declaration.Lhs[0].(*ast.Ident)
	if !ok || local.Name != identifier.Name {
		return nil
	}
	table, _ := ast.Unparen(declaration.Rhs[0]).(*ast.CompositeLit)
	return table
}
