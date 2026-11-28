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
	mark := func(table *ast.CompositeLit) {
		open := fset.PositionFor(table.Lbrace, false).Line
		closeLine := fset.PositionFor(table.Rbrace, false).Line
		for _, element := range table.Elts {
			first := fset.PositionFor(element.Pos(), false).Line
			last := fset.PositionFor(element.End(), false).Line
			for line := max(open+1, first); line <= min(closeLine-1, last); line++ {
				lines[line] = struct{}{}
			}
		}
	}
	for node := range ast.Preorder(body) {
		switch node := node.(type) {
		case *ast.RangeStmt:
			if table, ok := ast.Unparen(node.X).(*ast.CompositeLit); ok {
				mark(table)
			}
		case *ast.BlockStmt:
			markAdjacentTables(node.List, mark)
		case *ast.CaseClause:
			markAdjacentTables(node.Body, mark)
		case *ast.CommClause:
			markAdjacentTables(node.Body, mark)
		}
	}
	return len(lines)
}

func markAdjacentTables(statements []ast.Stmt, mark func(*ast.CompositeLit)) {
	var previous ast.Stmt
	for _, stmt := range statements {
		if loop, ok := stmt.(*ast.RangeStmt); ok {
			if table := adjacentCaseTable(previous, loop.X); table != nil {
				mark(table)
			}
		}
		previous = stmt
	}
}

func adjacentCaseTable(previous ast.Stmt, expr ast.Expr) *ast.CompositeLit {
	identifier, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	declaration, ok := previous.(*ast.AssignStmt)
	if !ok || declaration.Tok != token.DEFINE {
		return nil
	}
	if len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 {
		return nil
	}
	local, ok := declaration.Lhs[0].(*ast.Ident)
	if !ok || local.Name != identifier.Name {
		return nil
	}
	table, _ := ast.Unparen(declaration.Rhs[0]).(*ast.CompositeLit)
	return table
}
