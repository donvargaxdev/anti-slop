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

// testCaseLines counts case data in the body and its nested blocks. The loops
// and their assertions still count toward the limit.
func testCaseLines(fset *token.FileSet, body *ast.BlockStmt) int {
	lines := make(map[int]struct{})
	for node := range ast.Preorder(body) {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			continue
		}
		for table := range caseTables(block) {
			open := fset.PositionFor(table.Lbrace, false).Line
			closeLine := fset.PositionFor(table.Rbrace, false).Line
			for line := open + 1; line < closeLine; line++ {
				lines[line] = struct{}{}
			}
		}
	}
	return len(lines)
}

// caseTables finds inline tables and tables declared and ranged over in one block.
func caseTables(body *ast.BlockStmt) map[*ast.CompositeLit]struct{} {
	locals := make(map[string]*ast.CompositeLit)
	tables := make(map[*ast.CompositeLit]struct{})
	for _, stmt := range body.List {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt:
			if len(stmt.Lhs) == 1 && len(stmt.Rhs) == 1 {
				if id, ok := stmt.Lhs[0].(*ast.Ident); ok {
					locals[id.Name], _ = ast.Unparen(stmt.Rhs[0]).(*ast.CompositeLit)
				}
			}
		case *ast.DeclStmt:
			for _, spec := range stmt.Decl.(*ast.GenDecl).Specs {
				if value, ok := spec.(*ast.ValueSpec); ok && len(value.Names) == 1 && len(value.Values) == 1 {
					locals[value.Names[0].Name], _ = ast.Unparen(value.Values[0]).(*ast.CompositeLit)
				}
			}
		case *ast.RangeStmt:
			expr := ast.Unparen(stmt.X)
			table, _ := expr.(*ast.CompositeLit)
			if id, ok := expr.(*ast.Ident); ok {
				table = locals[id.Name]
			}
			if table != nil {
				tables[table] = struct{}{}
			}
		}
	}
	return tables
}
