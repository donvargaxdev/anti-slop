package tests

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Polling = &analysis.Analyzer{
	Name: "tests_polling",
	Doc:  "report loops that poll the wall clock in Go test code",
	Run:  runPolling,
}

func runPolling(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	deadlines := make(map[types.Object]bool)
	visitTests(pass, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			for i, value := range stmt.Rhs {
				if len(stmt.Lhs) == len(stmt.Rhs) && afterDeadline(info, value, deadlines) {
					if ident, ok := stmt.Lhs[i].(*ast.Ident); ok {
						deadlines[info.ObjectOf(ident)] = true
					}
				}
			}
		case *ast.ValueSpec:
			for i, value := range stmt.Values {
				if len(stmt.Names) == len(stmt.Values) && afterDeadline(info, value, deadlines) {
					deadlines[info.ObjectOf(stmt.Names[i])] = true
				}
			}
		}
	})
	visitTests(pass, func(node ast.Node) {
		var condition ast.Expr
		var body *ast.BlockStmt
		switch loop := node.(type) {
		case *ast.ForStmt:
			condition, body = loop.Cond, loop.Body
		case *ast.RangeStmt:
			body = loop.Body
		default:
			return
		}
		if readsClock(info, condition) || receivesDeadline(info, body, deadlines) {
			report(pass, node, "polling against the wall clock")
		}
	})
	return nil, nil
}

func readsClock(info *types.Info, condition ast.Expr) bool {
	if condition == nil {
		return false
	}
	return contains(condition, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		return ok && packageCall(info, call, "time", "Now", "Since", "Until")
	})
}

func afterDeadline(info *types.Info, expr ast.Expr, deadlines map[types.Object]bool) bool {
	switch value := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		return packageCall(info, value, "time", "After")
	case *ast.Ident:
		return deadlines[info.ObjectOf(value)]
	}
	return false
}

func receivesDeadline(info *types.Info, body *ast.BlockStmt, deadlines map[types.Object]bool) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch expr := node.(type) {
		case nil, *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			found = found || expr.Op == token.ARROW && afterDeadline(info, expr.X, deadlines)
		}
		return !found
	})
	return found
}
