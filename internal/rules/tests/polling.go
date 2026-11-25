package tests

import (
	"go/ast"
	"go/constant"
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
		var pollsInHeader bool
		var body *ast.BlockStmt
		switch loop := node.(type) {
		case *ast.ForStmt:
			body = loop.Body
			// The initializer takes one sample. The post statement can refresh it.
			pollsInHeader = readsClock(info, loop.Cond) || usesClockAssignment(info, loop.Cond, loop.Post)
		case *ast.RangeStmt:
			body = loop.Body
		default:
			return
		}
		if pollsInHeader || pollsInBody(info, body, receives) {
			report(pass, node, "polling against the wall clock")
		}
	})
	return nil, nil
}

type deadlineScan struct {
	info *types.Info
	// A nil channel map marks a path that branched away.
	channels map[types.Object]bool
	receives map[*ast.UnaryExpr]bool
	target   *branchTarget
}

type branchTarget struct {
	label     types.Object
	isLoop    bool
	breaks    map[types.Object]bool
	continues map[types.Object]bool
	parent    *branchTarget
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

func (scan deadlineScan) fork() *deadlineScan {
	scan.channels = maps.Clone(scan.channels)
	return &scan
}

func (scan deadlineScan) withTarget(label types.Object, isLoop bool) *deadlineScan {
	branch := scan.fork()
	branch.target = &branchTarget{label: label, isLoop: isLoop, parent: scan.target}
	return branch
}

func mergeDeadlines(dst, src map[types.Object]bool) map[types.Object]bool {
	if dst == nil {
		return maps.Clone(src)
	}
	maps.Copy(dst, src)
	return dst
}

func (scan *deadlineScan) record(root ast.Node) {
	if root == nil {
		return
	}
	ast.Inspect(root, scan.visit)
}

func (scan *deadlineScan) visit(node ast.Node) bool {
	if scan.channels == nil {
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
		scan.record(expr.Cond)
		other := scan.fork()
		scan.record(expr.Body)
		other.record(expr.Else)
		scan.channels = mergeDeadlines(scan.channels, other.channels)
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
		condition := scan.info.Types[expr.Cond].Value
		maySkip := expr.Cond != nil && (condition == nil || !constant.BoolVal(condition))
		scan.recordLoop(label, maySkip, expr.Body, expr.Post, expr.Cond)
	case *ast.RangeStmt:
		scan.record(expr.X)
		scan.recordLoop(label, scan.rangeMayBeEmpty(expr.X), expr.Body, nil, expr.Key, expr.Value)
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
		if expr.Op == token.ARROW && afterDeadline(scan.info, expr.X, scan.channels) {
			scan.receives[expr] = true
		}
		return true
	default:
		return true
	}
	return false
}

func (scan *deadlineScan) recordBranch(stmt *ast.BranchStmt) {
	label := scan.info.Uses[stmt.Label]
	continuing := stmt.Tok == token.CONTINUE
	for target := scan.target; target != nil; target = target.parent {
		matchesLabel := stmt.Label == nil || label != nil && target.label == label
		if !matchesLabel || continuing && !target.isLoop {
			continue
		}
		if continuing {
			target.continues = mergeDeadlines(target.continues, scan.channels)
		} else {
			target.breaks = mergeDeadlines(target.breaks, scan.channels)
		}
		scan.channels = nil
		return
	}
}

func (scan *deadlineScan) recordSelect(body *ast.BlockStmt, label types.Object) {
	entry := scan.withTarget(label, false)
	scan.channels = nil
	// Each clause starts from the same state, and only the selected clause assigns values.
	for _, stmt := range body.List {
		branch := entry.fork()
		branch.record(stmt)
		scan.channels = mergeDeadlines(scan.channels, branch.channels)
	}
	scan.channels = mergeDeadlines(scan.channels, entry.target.breaks)
}

func (scan *deadlineScan) recordSwitch(body *ast.BlockStmt, label types.Object) {
	entry := scan.withTarget(label, false)
	scan.channels = nil
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
		maps.Copy(branch.channels, fallingThrough)
		fallsThrough := false
		for _, stmt := range clause.Body {
			branch.record(stmt)
			fallsThrough = fallsThrough || isFallthrough(stmt)
		}
		if fallsThrough {
			fallingThrough = branch.channels
			continue
		}
		fallingThrough = nil
		scan.channels = mergeDeadlines(scan.channels, branch.channels)
	}
	if !hasDefault {
		scan.channels = mergeDeadlines(scan.channels, entry.channels)
	}
	scan.channels = mergeDeadlines(scan.channels, entry.target.breaks)
}

func isFallthrough(stmt ast.Stmt) bool {
	for label, ok := stmt.(*ast.LabeledStmt); ok; label, ok = stmt.(*ast.LabeledStmt) {
		stmt = label.Stmt
	}
	branch, ok := stmt.(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

func (scan deadlineScan) rangeMayBeEmpty(expr ast.Expr) bool {
	if value := scan.info.Types[expr].Value; value != nil {
		switch value.Kind() {
		case constant.Int:
			return constant.Sign(value) <= 0
		case constant.String:
			return constant.StringVal(value) == ""
		}
	}
	underlying := scan.info.TypeOf(expr).Underlying()
	if pointer, ok := underlying.(*types.Pointer); ok {
		underlying = pointer.Elem().Underlying()
	}
	if array, ok := underlying.(*types.Array); ok {
		return array.Len() == 0
	}
	return true
}

func (scan *deadlineScan) recordLoop(label types.Object, maySkip bool, body *ast.BlockStmt, post ast.Stmt, beforeBody ...ast.Node) {
	entry := scan.withTarget(label, true)
	for {
		iteration := entry.fork()
		for _, node := range beforeBody {
			iteration.record(node)
		}
		iteration.record(body)
		// A continue bypasses the rest of the body and still runs the post statement.
		iteration.channels = mergeDeadlines(iteration.channels, entry.target.continues)
		iteration.record(post)
		// A deadline assigned on one iteration can reach a receive on the next.
		previous := len(entry.channels)
		maps.Copy(entry.channels, iteration.channels)
		if len(entry.channels) == previous {
			scan.channels = iteration.channels
			if maySkip {
				// The loop can leave incoming deadlines untouched by skipping its body.
				scan.channels = mergeDeadlines(scan.channels, entry.channels)
			}
			// Break paths bypass the remaining body and post statement.
			scan.channels = mergeDeadlines(scan.channels, entry.target.breaks)
			return
		}
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
			(readsClock(info, stmt.Cond) || usesClockAssignment(info, stmt.Cond, stmt.Init)) {
			return true
		}
	}
	return false
}

func usesClockAssignment(info *types.Info, condition ast.Expr, stmt ast.Stmt) bool {
	assignment, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
		return false
	}
	for i, value := range assignment.Rhs {
		ident, ok := ast.Unparen(assignment.Lhs[i]).(*ast.Ident)
		if !ok || ident.Name == "_" || !readsClock(info, value) {
			continue
		}
		object := info.ObjectOf(ident)
		if contains(condition, func(node ast.Node) bool {
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
