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
		if readsClock(info, condition) || pollsInBody(info, body, deadlines) {
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

func pollsInBody(info *types.Info, body *ast.BlockStmt, deadlines map[types.Object]bool) bool {
	found := false
	ast.PreorderStack(body, nil, func(node ast.Node, parents []ast.Node) bool {
		if found {
			return false
		}
		switch expr := node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			found = expr.Op == token.ARROW && afterDeadline(info, expr.X, deadlines)
		case *ast.ReturnStmt:
			found = guardedByClock(info, parents)
		case *ast.BranchStmt:
			found = expr.Tok == token.BREAK && breaksLoop(info, expr, body, parents) && guardedByClock(info, parents)
		}
		return !found
	})
	return found
}

func guardedByClock(info *types.Info, parents []ast.Node) bool {
	for _, node := range parents {
		if stmt, ok := node.(*ast.IfStmt); ok && readsClock(info, stmt.Cond) {
			return true
		}
	}
	return false
}

func breaksLoop(info *types.Info, branch *ast.BranchStmt, body *ast.BlockStmt, parents []ast.Node) bool {
	if branch.Label != nil {
		// A break to a label outside the body also leaves this loop.
		return info.Uses[branch.Label].Pos() < body.Pos()
	}
	for _, node := range parents {
		switch node.(type) {
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		}
	}
	return true
}
