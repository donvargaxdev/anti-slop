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
	visitTestFiles(pass, func(file *ast.File) {
		ast.PreorderStack(file, nil, func(node ast.Node, parents []ast.Node) bool {
			if pollsInLoop(info, node, parents, receives) {
				report(pass, node, "polling against the wall clock")
			}
			return true
		})
	})
	return nil, nil
}

type pollingScan struct {
	info *types.Info
	// A nil map marks a path that branched away.
	tracked     map[types.Object]bool
	dependsOn   func(ast.Expr, map[types.Object]bool) bool
	receives    map[*ast.UnaryExpr]bool
	clockGuards map[*ast.IfStmt]bool
	target      *branchTarget[bool]
}

type branchTarget[T any] struct {
	label     types.Object
	isLoop    bool
	breaks    map[types.Object]T
	continues map[types.Object]T
	parent    *branchTarget[T]
}

func deadlineReceives(pass *analysis.Pass) map[*ast.UnaryExpr]bool {
	scan := pollingScan{
		info:    pass.TypesInfo,
		tracked: make(map[types.Object]bool),
		dependsOn: func(expr ast.Expr, tracked map[types.Object]bool) bool {
			return afterDeadline(pass.TypesInfo, expr, tracked)
		},
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

func (scan pollingScan) fork() *pollingScan {
	scan.tracked = maps.Clone(scan.tracked)
	return &scan
}

func (scan pollingScan) withTarget(label types.Object, isLoop bool) *pollingScan {
	branch := scan.fork()
	branch.target = &branchTarget[bool]{label: label, isLoop: isLoop, parent: scan.target}
	return branch
}

func mergeTracked(dst, src map[types.Object]bool) map[types.Object]bool {
	if dst == nil {
		return maps.Clone(src)
	}
	maps.Copy(dst, src)
	return dst
}

func (scan *pollingScan) record(root ast.Node) {
	if root == nil {
		return
	}
	ast.Inspect(root, scan.visit)
}

func (scan *pollingScan) visit(node ast.Node) bool {
	if scan.tracked == nil {
		return false
	}
	var label types.Object
	for labeled, ok := node.(*ast.LabeledStmt); ok; labeled, ok = node.(*ast.LabeledStmt) {
		label = scan.info.Defs[labeled.Label]
		node = labeled.Stmt
	}
	switch expr := node.(type) {
	case *ast.FuncLit:
		body := scan.fork()
		body.target = nil
		body.record(expr.Body)
	case *ast.IfStmt:
		scan.record(expr.Init)
		if scan.clockGuards != nil && scan.dependsOn(expr.Cond, scan.tracked) {
			scan.clockGuards[expr] = true
		}
		scan.record(expr.Cond)
		other := scan.fork()
		scan.record(expr.Body)
		other.record(expr.Else)
		scan.tracked = mergeTracked(scan.tracked, other.tracked)
	case *ast.SwitchStmt:
		scan.record(expr.Init)
		scan.record(expr.Tag)
		scan.recordSwitch(expr.Body, label)
	case *ast.TypeSwitchStmt:
		scan.record(expr.Init)
		scan.record(expr.Assign)
		scan.recordSwitch(expr.Body, label)
	case *ast.SelectStmt:
		scan.recordSelect(expr.Body, label)
	case *ast.ForStmt:
		scan.record(expr.Init)
		scan.recordLoop(label, forMaySkip(scan.info, expr.Cond), expr.Body, expr.Post, func(iteration *pollingScan) {
			iteration.record(expr.Cond)
		})
	case *ast.RangeStmt:
		scan.record(expr.X)
		scan.recordLoop(label, rangeMayBeEmpty(scan.info, expr.X), expr.Body, nil, func(iteration *pollingScan) {
			iteration.recordRangeAssignment(expr)
		})
	case *ast.ReturnStmt:
		for _, result := range expr.Results {
			scan.record(result)
		}
		scan.tracked = nil
	case *ast.BranchStmt:
		switch expr.Tok {
		case token.BREAK, token.CONTINUE:
			scan.recordBranch(expr)
		}
	case *ast.AssignStmt:
		scan.recordAssignment(expr)
	case *ast.ValueSpec:
		values := scan.values(expr.Values, len(expr.Names))
		for i, name := range expr.Names {
			scan.store(name, values[i])
		}
	case *ast.UnaryExpr:
		if scan.receives != nil && expr.Op == token.ARROW && afterDeadline(scan.info, expr.X, scan.tracked) {
			scan.receives[expr] = true
		}
		return true
	default:
		return true
	}
	return false
}

func (scan *pollingScan) recordBranch(stmt *ast.BranchStmt) {
	target := scan.target.destination(scan.info, stmt)
	if target == nil {
		return
	}
	if stmt.Tok == token.CONTINUE {
		target.continues = mergeTracked(target.continues, scan.tracked)
	} else {
		target.breaks = mergeTracked(target.breaks, scan.tracked)
	}
	scan.tracked = nil
}

func (target *branchTarget[T]) destination(info *types.Info, stmt *ast.BranchStmt) *branchTarget[T] {
	label := info.Uses[stmt.Label]
	continuing := stmt.Tok == token.CONTINUE
	for ; target != nil; target = target.parent {
		matchesLabel := stmt.Label == nil || label != nil && target.label == label
		if matchesLabel && (!continuing || target.isLoop) {
			return target
		}
	}
	return nil
}

func (scan *pollingScan) recordSelect(body *ast.BlockStmt, label types.Object) {
	entry := scan.withTarget(label, false)
	scan.tracked = nil
	// Each clause starts from the same state, and only the selected clause assigns values.
	for _, stmt := range body.List {
		branch := entry.fork()
		branch.record(stmt)
		scan.tracked = mergeTracked(scan.tracked, branch.tracked)
	}
	scan.tracked = mergeTracked(scan.tracked, entry.target.breaks)
}

func (scan *pollingScan) recordSwitch(body *ast.BlockStmt, label types.Object) {
	entry := scan.withTarget(label, false)
	scan.tracked = nil
	hasDefault := false
	var fallingThrough map[types.Object]bool
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		hasDefault = hasDefault || clause.List == nil
		branch := entry.fork()
		for _, expr := range clause.List {
			branch.record(expr)
		}
		// Fallthrough enters the case body without testing the case expressions.
		maps.Copy(branch.tracked, fallingThrough)
		fallsThrough := false
		for _, stmt := range clause.Body {
			branch.record(stmt)
			fallsThrough = fallsThrough || isFallthrough(stmt)
		}
		if fallsThrough {
			fallingThrough = branch.tracked
			continue
		}
		fallingThrough = nil
		scan.tracked = mergeTracked(scan.tracked, branch.tracked)
	}
	if !hasDefault {
		scan.tracked = mergeTracked(scan.tracked, entry.tracked)
	}
	scan.tracked = mergeTracked(scan.tracked, entry.target.breaks)
}

func isFallthrough(stmt ast.Stmt) bool {
	branch, ok := unlabel(stmt).(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

func (scan *pollingScan) recordLoop(label types.Object, maySkip bool, body *ast.BlockStmt, post ast.Stmt, beforeBody func(*pollingScan)) {
	entry := scan.withTarget(label, true)
	for {
		iteration := entry.fork()
		beforeBody(iteration)
		iteration.record(body)
		// A continue bypasses the rest of the body and still runs the post statement.
		iteration.tracked = mergeTracked(iteration.tracked, entry.target.continues)
		iteration.record(post)
		// A value assigned on one iteration can be used on the next.
		previous := len(entry.tracked)
		maps.Copy(entry.tracked, iteration.tracked)
		if len(entry.tracked) == previous {
			scan.tracked = iteration.tracked
			if maySkip {
				// The loop can leave incoming values untouched by skipping its body.
				scan.tracked = mergeTracked(scan.tracked, entry.tracked)
			}
			// Break paths bypass the remaining body and post statement.
			scan.tracked = mergeTracked(scan.tracked, entry.target.breaks)
			return
		}
	}
}

func (scan pollingScan) recordRangeAssignment(stmt *ast.RangeStmt) {
	targets := [...]ast.Expr{stmt.Key, stmt.Value}
	for _, target := range targets {
		scan.record(target)
	}
	// Each iteration replaces the targets with values supplied by the range.
	for _, target := range targets {
		if name, ok := ast.Unparen(target).(*ast.Ident); ok {
			scan.store(name, false)
		}
	}
}

func (scan pollingScan) recordAssignment(stmt *ast.AssignStmt) {
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
func (scan pollingScan) values(exprs []ast.Expr, count int) []bool {
	values := make([]bool, count)
	for i, expr := range exprs {
		scan.record(expr)
		if len(exprs) == count {
			values[i] = scan.dependsOn(expr, scan.tracked)
		}
	}
	return values
}

func (scan pollingScan) store(name *ast.Ident, tracked bool) {
	object := scan.info.ObjectOf(name)
	if tracked {
		scan.tracked[object] = true
	} else {
		delete(scan.tracked, object)
	}
}

func pollsInLoop(info *types.Info, node ast.Node, parents []ast.Node, receives map[*ast.UnaryExpr]bool) bool {
	var body *ast.BlockStmt
	var condition ast.Expr
	var post ast.Stmt
	switch loop := node.(type) {
	case *ast.ForStmt:
		body, condition, post = loop.Body, loop.Cond, loop.Post
	case *ast.RangeStmt:
		body = loop.Body
	default:
		return false
	}
	if readsClock(info, condition) {
		return true
	}

	target := &branchTarget[bool]{isLoop: true}
	if labeled, ok := parents[len(parents)-1].(*ast.LabeledStmt); ok {
		target.label = info.Defs[labeled.Label]
	}
	// Start with an empty state so a fixed snapshot taken before the loop does
	// not count as polling. Record each guard with the values that reach it.
	scan := pollingScan{
		info:    info,
		tracked: make(map[types.Object]bool),
		dependsOn: func(expr ast.Expr, tracked map[types.Object]bool) bool {
			return readsClock(info, expr) || usesTrackedValue(info, expr, tracked)
		},
		clockGuards: make(map[*ast.IfStmt]bool),
		target:      target,
	}
	scan.record(body)
	if condition != nil {
		scan.tracked = mergeTracked(scan.tracked, target.continues)
		scan.record(post)
		if usesTrackedValue(info, condition, scan.tracked) {
			return true
		}
	}
	return pollsInBody(info, body, receives, scan.clockGuards)
}

func usesTrackedValue(info *types.Info, expr ast.Expr, tracked map[types.Object]bool) bool {
	return contains(expr, func(node ast.Node) bool {
		name, ok := node.(*ast.Ident)
		return ok && tracked[info.Uses[name]]
	})
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

func pollsInBody(info *types.Info, body *ast.BlockStmt, receives map[*ast.UnaryExpr]bool, clockGuards map[*ast.IfStmt]bool) bool {
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
			found = guardedByClock(parents, clockGuards)
		case *ast.BranchStmt:
			found = expr.Tok == token.BREAK && breaksLoop(info, expr, body, parents) && guardedByClock(parents, clockGuards)
		case *ast.ExprStmt:
			if call, ok := ast.Unparen(expr.X).(*ast.CallExpr); ok &&
				testingMethod(info, call, "Fatal", "Fatalf", "FailNow") {
				found = guardedByClock(parents, clockGuards)
			}
		}
		return !found
	})
	return found
}

func guardedByClock(parents []ast.Node, clockGuards map[*ast.IfStmt]bool) bool {
	for _, node := range parents {
		if stmt, ok := node.(*ast.IfStmt); ok && clockGuards[stmt] {
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
