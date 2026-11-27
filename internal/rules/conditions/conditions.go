// Package conditions reports long compound conditions in Go source.
package conditions

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

const name = "conditions"

// Analyzer finds if and for conditions that should be split into named booleans.
var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "report if and for conditions with more than two logical operators",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for node := range ast.Preorder(file) {
			var cond ast.Expr
			switch stmt := node.(type) {
			case *ast.IfStmt:
				cond = stmt.Cond
			case *ast.ForStmt:
				cond = stmt.Cond
			}
			if cond != nil && logicalOperators(cond) > 2 {
				pass.Report(analysis.Diagnostic{
					Pos: cond.Pos(), Category: name, Message: "split this condition into named booleans",
				})
			}
		}
	}
	return nil, nil
}

func logicalOperators(expr ast.Expr) int {
	count := 0
	ast.Inspect(expr, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			switch node.Op {
			case token.LAND, token.LOR:
				count++
			}
		}
		return true
	})
	return count
}
