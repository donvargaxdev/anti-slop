package tests

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"
)

var Polling = &analysis.Analyzer{
	Name: "tests_polling",
	Doc:  "report loops that poll the wall clock in Go test code",
	Run:  runPolling,
}

func runPolling(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	receives := deadlineReceives(pass)
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
		if readsClock(info, condition) || pollsInBody(info, body, receives) {
			report(pass, node, "polling against the wall clock")
		}
	})
	return nil, nil
}

type deadlineScan struct {
	info     *types.Info
	channels map[types.Object]bool
	receives map[*ast.UnaryExpr]bool
}

func deadlineReceives(pass *analysis.Pass) map[*ast.UnaryExpr]bool {
	scan := deadlineScan{
		info:     pass.TypesInfo,
		channels: make(map[types.Object]bool),
		receives: make(map[*ast.UnaryExpr]bool),
	}
	visitTestFiles(pass, func(file *ast.File) {
		for _, decl := range file.Decls {
			if vars, ok := decl.(*ast.GenDecl); ok && vars.Tok == token.VAR {
				scan.record(vars)
			}
		}
	})
	visitTestFiles(pass, func(file *ast.File) {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				scan.fork().record(fn.Body)
			}
		}
	})
	return scan.receives
}

func (scan deadlineScan) fork() deadlineScan {
	scan.channels = maps.Clone(scan.channels)
	return scan
}

func (scan deadlineScan) record(root ast.Node) {
	if root == nil {
		return
	}
	ast.Inspect(root, scan.visit)
}

func (scan deadlineScan) visit(node ast.Node) bool {
	switch expr := node.(type) {
	case *ast.FuncLit:
		scan.fork().record(expr.Body)
	case *ast.IfStmt:
		scan.record(expr.Init)
		scan.record(expr.Cond)
		other := scan.fork()
		scan.record(expr.Body)
		other.record(expr.Else)
		maps.Copy(scan.channels, other.channels)
	case *ast.ForStmt:
		scan.record(expr.Init)
		scan.recordLoop(expr.Cond, expr.Body, expr.Post)
	case *ast.RangeStmt:
		scan.record(expr.X)
		scan.recordLoop(expr.Key, expr.Value, expr.Body)
	case *ast.AssignStmt:
		scan.recordAssignment(expr)
	case *ast.ValueSpec:
		values := scan.values(expr.Values, len(expr.Names))
		for i, name := range expr.Names {
			scan.store(name, values[i])
		}
	case *ast.UnaryExpr:
		if expr.Op == token.ARROW && afterDeadline(scan.info, expr.X, scan.channels) {
			scan.receives[expr] = true
		}
		return true
	default:
		return true
	}
	return false
}

func (scan deadlineScan) recordLoop(parts ...ast.Node) {
	entry := scan.fork()
	for {
		for _, part := range parts {
			scan.record(part)
		}
		// A deadline assigned on one iteration can reach a receive on the next.
		previous := len(entry.channels)
		maps.Copy(entry.channels, scan.channels)
		if len(entry.channels) == previous {
			return
		}
		maps.Copy(scan.channels, entry.channels)
	}
}

func (scan deadlineScan) recordAssignment(stmt *ast.AssignStmt) {
	for _, lhs := range stmt.Lhs {
		scan.record(lhs)
	}
	values := scan.values(stmt.Rhs, len(stmt.Lhs))
	for i, lhs := range stmt.Lhs {
		if name, ok := ast.Unparen(lhs).(*ast.Ident); ok {
			scan.store(name, values[i])
		}
	}
}

// Evaluate the right-hand sides before replacing any of the assigned values.
func (scan deadlineScan) values(exprs []ast.Expr, count int) []bool {
	values := make([]bool, count)
	for i, expr := range exprs {
		scan.record(expr)
		if len(exprs) == count {
			values[i] = afterDeadline(scan.info, expr, scan.channels)
		}
	}
	return values
}

func (scan deadlineScan) store(name *ast.Ident, deadline bool) {
	object := scan.info.ObjectOf(name)
	if deadline {
		scan.channels[object] = true
	} else {
		delete(scan.channels, object)
	}
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

func pollsInBody(info *types.Info, body *ast.BlockStmt, receives map[*ast.UnaryExpr]bool) bool {
	found := false
	ast.PreorderStack(body, nil, func(node ast.Node, parents []ast.Node) bool {
		if found {
			return false
		}
		switch expr := node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			found = receives[expr]
		case *ast.ReturnStmt:
			found = guardedByClock(info, parents)
		case *ast.BranchStmt:
			found = expr.Tok == token.BREAK && breaksLoop(info, expr, body, parents) && guardedByClock(info, parents)
		case *ast.ExprStmt:
			if call, ok := ast.Unparen(expr.X).(*ast.CallExpr); ok &&
				testingMethod(info, call, "Fatal", "Fatalf", "FailNow") {
				found = guardedByClock(info, parents)
			}
		}
		return !found
	})
	return found
}

func guardedByClock(info *types.Info, parents []ast.Node) bool {
	for _, node := range parents {
		if stmt, ok := node.(*ast.IfStmt); ok &&
			(readsClock(info, stmt.Cond) || usesClockInitializer(info, stmt)) {
			return true
		}
	}
	return false
}

func usesClockInitializer(info *types.Info, stmt *ast.IfStmt) bool {
	assignment, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
		return false
	}
	for i, value := range assignment.Rhs {
		ident, ok := ast.Unparen(assignment.Lhs[i]).(*ast.Ident)
		if !ok || ident.Name == "_" || !readsClock(info, value) {
			continue
		}
		object := info.ObjectOf(ident)
		if contains(stmt.Cond, func(node ast.Node) bool {
			use, ok := node.(*ast.Ident)
			return ok && info.Uses[use] == object
		}) {
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
