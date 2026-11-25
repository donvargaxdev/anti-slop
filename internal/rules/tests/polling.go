package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"slices"

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
	pollingFlow
	dependsOn       func(ast.Expr, map[types.Object]bool) bool
	receives        map[*ast.UnaryExpr]bool
	clockGuards     map[ast.Node]bool
	callResults     map[*ast.CallExpr]bool
	target          *branchTarget[pollingFlow]
	continuations   *pollingContinuations
	returns         *pollingFlow
	result          *types.Var
	clockControlled bool
}

type pollingFlow struct {
	// A nil map marks a path that branched away.
	tracked map[types.Object]bool
	// A clock-controlled continue can bypass this path in the current iteration.
	bypassed bool
}

type pollingContinuations struct {
	// Shared by branch scans so a guard can detect continues to the analyzed loop.
	count        int
	bypassesExit bool
}

type branchTarget[T any] struct {
	label     types.Object
	isLoop    bool
	breaks    T
	continues T
	parent    *branchTarget[T]
}

func deadlineReceives(pass *analysis.Pass) map[*ast.UnaryExpr]bool {
	callResults := make(map[*ast.CallExpr]bool)
	scan := pollingScan{
		info:        pass.TypesInfo,
		pollingFlow: pollingFlow{tracked: make(map[types.Object]bool)},
		dependsOn: func(expr ast.Expr, tracked map[types.Object]bool) bool {
			return afterDeadline(pass.TypesInfo, expr, tracked, callResults)
		},
		receives:    make(map[*ast.UnaryExpr]bool),
		callResults: callResults,
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
	branch.target = &branchTarget[pollingFlow]{label: label, isLoop: isLoop, parent: scan.target}
	return branch
}

func mergePollingFlows(dst, src pollingFlow) pollingFlow {
	if dst.tracked == nil {
		src.tracked = maps.Clone(src.tracked)
		return src
	}
	maps.Copy(dst.tracked, src.tracked)
	dst.bypassed = dst.bypassed || src.bypassed
	return dst
}

func (scan *pollingScan) stopPath() {
	scan.pollingFlow = pollingFlow{}
}

func (scan *pollingScan) continueCount() int {
	if scan.continuations == nil {
		return 0
	}
	return scan.continuations.count
}

func (scan *pollingScan) recordExit() {
	if scan.bypassed {
		scan.continuations.bypassesExit = true
	}
}

func (scan *pollingScan) record(root ast.Node) {
	if root == nil {
		return
	}
	ast.PreorderStack(root, nil, func(node ast.Node, parents []ast.Node) bool {
		var parent ast.Node
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		}
		return scan.visit(node, parent)
	})
}

func (scan *pollingScan) visit(node, parent ast.Node) bool {
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
		body.clockControlled = false
		body.recordLiteral(expr, nil, nil)
	case *ast.CallExpr:
		literal := immediateLiteral(expr, parent)
		if literal == nil {
			return true
		}
		scan.recordImmediateCall(expr, literal)
	case *ast.BinaryExpr:
		if expr.Op != token.LAND && expr.Op != token.LOR {
			return true
		}
		scan.recordShortCircuit(expr)
	case *ast.IfStmt:
		scan.record(expr.Init)
		clockGuard := scan.recordClockGuard(expr.Cond)
		if clockGuard {
			scan.clockGuards[expr] = true
		}
		before := scan.continueCount()
		outerControl := scan.clockControlled
		scan.clockControlled = outerControl || clockGuard
		other := scan.fork()
		scan.record(expr.Body)
		other.record(expr.Else)
		scan.clockControlled = outerControl
		scan.pollingFlow = mergePollingFlows(scan.pollingFlow, other.pollingFlow)
		scan.markBypassed(clockGuard && scan.continueCount() != before)
	case *ast.SwitchStmt:
		scan.record(expr.Init)
		tagReadsClock := scan.recordClockGuard(expr.Tag)
		scan.recordSwitch(expr.Body, label, tagReadsClock)
	case *ast.TypeSwitchStmt:
		scan.record(expr.Init)
		scan.record(expr.Assign)
		scan.recordSwitch(expr.Body, label, false)
	case *ast.SelectStmt:
		scan.recordSelect(expr.Body, label)
	case *ast.ForStmt:
		scan.record(expr.Init)
		scan.recordLoop(label, forBounds(scan.info, expr.Cond), expr.Body, expr.Post, func(iteration *pollingScan) {
			iteration.record(expr.Cond)
		})
	case *ast.RangeStmt:
		scan.record(expr.X)
		scan.recordLoop(label, rangeBounds(scan.info, expr.X), expr.Body, nil, func(iteration *pollingScan) {
			iteration.recordRangeAssignment(expr)
		})
	case *ast.ReturnStmt:
		for _, result := range expr.Results {
			scan.record(result)
		}
		if scan.result != nil {
			dependsOnClock := scan.tracked[scan.result]
			if len(expr.Results) == 1 {
				dependsOnClock = scan.dependsOn(expr.Results[0], scan.tracked)
			}
			scan.store(scan.result, dependsOnClock)
		}
		scan.recordExit()
		if scan.returns != nil {
			*scan.returns = mergePollingFlows(*scan.returns, scan.pollingFlow)
		}
		scan.stopPath()
	case *ast.ExprStmt:
		if !endsTest(scan.info, expr.X) {
			return true
		}
		scan.record(expr.X)
		scan.recordExit()
		scan.stopPath()
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
			scan.store(scan.info.ObjectOf(name), values[i])
		}
	case *ast.UnaryExpr:
		if scan.receives == nil || expr.Op != token.ARROW {
			return true
		}
		scan.record(expr.X)
		if scan.dependsOn(expr.X, scan.tracked) {
			scan.receives[expr] = true
		}
	default:
		return true
	}
	return false
}

func (scan *pollingScan) recordImmediateCall(call *ast.CallExpr, literal *ast.FuncLit) {
	arguments := scan.values(call.Args, len(call.Args))
	if scan.tracked == nil {
		return
	}

	body := scan.fork()
	signature := scan.info.TypeOf(literal).(*types.Signature)
	params := signature.Params()
	fixed := params.Len()
	if signature.Variadic() {
		fixed--
	}
	for i := range min(fixed, len(arguments)) {
		// A tuple argument has no separately tracked value for each result.
		if _, tuple := scan.info.TypeOf(call.Args[i]).(*types.Tuple); arguments[i] && !tuple {
			body.tracked[params.At(i)] = true
		}
	}
	var result *types.Var
	if signature.Results().Len() == 1 {
		result = signature.Results().At(0)
	}
	var returns pollingFlow
	body.recordLiteral(literal, &returns, result)
	// Explicit returns and fallthrough both resume the caller.
	resumed := mergePollingFlows(body.pollingFlow, returns)
	if result != nil {
		scan.callResults[call] = returns.tracked[result]
		delete(resumed.tracked, result)
	}
	if resumed.tracked == nil {
		scan.recordExit()
		scan.stopPath()
		return
	}
	for i := range fixed {
		delete(resumed.tracked, params.At(i))
	}
	scan.tracked = resumed.tracked
}

func (scan *pollingScan) recordLiteral(literal *ast.FuncLit, returns *pollingFlow, result *types.Var) {
	scan.target = nil
	scan.continuations = nil
	scan.bypassed = false
	scan.returns = returns
	scan.result = result
	scan.record(literal.Body)
}

func (scan *pollingScan) recordShortCircuit(expr *ast.BinaryExpr) {
	clockGuard := scan.recordClockGuard(expr.X)
	if value := scan.info.Types[expr.X].Value; value != nil {
		if constant.BoolVal(value) == (expr.Op == token.LAND) {
			scan.record(expr.Y)
		}
		return
	}
	right := scan.fork()
	right.clockControlled = scan.clockControlled || clockGuard
	right.record(expr.Y)
	scan.pollingFlow = mergePollingFlows(scan.pollingFlow, right.pollingFlow)
}

func (scan *pollingScan) markBypassed(guardedContinue bool) {
	if guardedContinue && scan.tracked != nil {
		scan.bypassed = true
	}
}

func (scan *pollingScan) recordBranch(stmt *ast.BranchStmt) {
	target := scan.target.destination(scan.info, stmt)
	if target == nil {
		return
	}
	if stmt.Tok == token.CONTINUE {
		target.continues = mergePollingFlows(target.continues, scan.pollingFlow)
		if scan.continuations != nil && target.parent == nil {
			scan.continuations.count++
		}
	} else {
		target.breaks = mergePollingFlows(target.breaks, scan.pollingFlow)
		if target.parent == nil {
			scan.recordExit()
		}
	}
	scan.stopPath()
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

func (scan *pollingScan) recordClockGuard(expr ast.Expr) bool {
	// Read incoming values before the guard runs and returned values after it runs.
	readsClock := scan.clockGuards != nil && usesClockValue(scan.info, expr, scan.tracked, nil)
	scan.record(expr)
	return readsClock || scan.clockGuards != nil && usesClockValue(scan.info, expr, nil, scan.callResults)
}

func (scan *pollingScan) recordLoop(label types.Object, bounds loopBounds, body *ast.BlockStmt, post ast.Stmt, beforeBody func(*pollingScan)) {
	if bounds.maxIterations == 0 {
		return
	}
	entry := scan.withTarget(label, true)
	for count := int64(1); ; count++ {
		iteration := entry.fork()
		beforeBody(iteration)
		iteration.record(body)
		// A continue bypasses the rest of the body and still runs the post statement.
		iteration.pollingFlow = mergePollingFlows(iteration.pollingFlow, entry.target.continues)
		iteration.record(post)
		// A value assigned on one iteration can be used on the next.
		previous := len(entry.tracked)
		maps.Copy(entry.tracked, iteration.tracked)
		if count == bounds.maxIterations || len(entry.tracked) == previous {
			scan.pollingFlow = iteration.pollingFlow
			if bounds.maySkip {
				// The loop can leave incoming values untouched by skipping its body.
				scan.pollingFlow = mergePollingFlows(scan.pollingFlow, entry.pollingFlow)
			}
			// Break paths bypass the remaining body and post statement.
			scan.pollingFlow = mergePollingFlows(scan.pollingFlow, entry.target.breaks)
			return
		}
	}
}

func (scan *pollingScan) recordRangeAssignment(stmt *ast.RangeStmt) {
	targets := [...]ast.Expr{stmt.Key, stmt.Value}
	for _, target := range targets {
		scan.record(target)
	}
	// Each iteration replaces the targets with values supplied by the range.
	for _, target := range targets {
		if name, ok := ast.Unparen(target).(*ast.Ident); ok {
			scan.store(scan.info.ObjectOf(name), false)
		}
	}
}

func (scan *pollingScan) recordAssignment(stmt *ast.AssignStmt) {
	for _, lhs := range stmt.Lhs {
		scan.record(lhs)
	}
	values := scan.values(stmt.Rhs, len(stmt.Lhs))
	for i, lhs := range stmt.Lhs {
		if name, ok := ast.Unparen(lhs).(*ast.Ident); ok {
			object := scan.info.ObjectOf(name)
			compound := stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE
			scan.store(object, values[i] || compound && scan.tracked[object])
		}
	}
}

// Evaluate the right-hand sides before replacing any of the assigned values.
func (scan *pollingScan) values(exprs []ast.Expr, count int) []bool {
	values := make([]bool, count)
	for i, expr := range exprs {
		scan.record(expr)
		if len(exprs) == count {
			values[i] = scan.dependsOn(expr, scan.tracked)
		}
	}
	return values
}

func (scan pollingScan) store(object types.Object, tracked bool) {
	if scan.tracked == nil {
		return
	}
	// A clock guard can select which value is stored even if it is constant.
	if tracked || scan.clockControlled {
		scan.tracked[object] = true
	} else {
		delete(scan.tracked, object)
	}
}

func pollsInLoop(info *types.Info, node ast.Node, parents []ast.Node, receives map[*ast.UnaryExpr]bool) bool {
	var body *ast.BlockStmt
	var condition ast.Expr
	var post ast.Stmt
	var rangeLoop *ast.RangeStmt
	var bounds loopBounds
	switch loop := node.(type) {
	case *ast.ForStmt:
		body, condition, post = loop.Body, loop.Cond, loop.Post
		bounds = forBounds(info, loop.Cond)
	case *ast.RangeStmt:
		body = loop.Body
		rangeLoop = loop
		bounds = rangeBounds(info, loop.X)
	default:
		return false
	}
	if bounds.maxIterations == 0 {
		return false
	}
	if pollsIn(info, condition, receives, nil) || pollsIn(info, post, receives, nil) {
		return true
	}

	var label types.Object
	if labeled, ok := parents[len(parents)-1].(*ast.LabeledStmt); ok {
		label = info.Defs[labeled.Label]
	}
	// Start with an empty state so a fixed snapshot taken before the loop does
	// not count as polling. Record each guard with the values that reach it.
	callResults := make(map[*ast.CallExpr]bool)
	scan := pollingScan{
		info:        info,
		pollingFlow: pollingFlow{tracked: make(map[types.Object]bool)},
		dependsOn: func(expr ast.Expr, tracked map[types.Object]bool) bool {
			return usesClockValue(info, expr, tracked, callResults)
		},
		clockGuards:   make(map[ast.Node]bool),
		callResults:   callResults,
		continuations: &pollingContinuations{},
	}
	clockInHeader := false
	scan.recordLoop(label, bounds, body, post, func(iteration *pollingScan) {
		if rangeLoop != nil {
			iteration.recordRangeAssignment(rangeLoop)
		}
		clockInHeader = iteration.recordClockGuard(condition) || clockInHeader
	})
	return clockInHeader || scan.continuations.bypassesExit || pollsIn(info, body, receives, scan.clockGuards)
}

func usesClockValue(info *types.Info, expr ast.Expr, tracked map[types.Object]bool, calls map[*ast.CallExpr]bool) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if node == nil || found {
			return false
		}
		switch value := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.Ident:
			found = tracked[info.Uses[value]]
		case *ast.CallExpr:
			if immediateLiteral(value, nil) != nil {
				// The result may depend on only some of the arguments.
				found = calls[value]
				return false
			}
			found = packageCall(info, value, "time", "Now", "Since", "Until")
		}
		return !found
	})
	return found
}

func afterDeadline(info *types.Info, expr ast.Expr, deadlines map[types.Object]bool, calls map[*ast.CallExpr]bool) bool {
	switch value := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		return calls[value] || packageCall(info, value, "time", "After")
	case *ast.Ident:
		return deadlines[info.ObjectOf(value)]
	}
	return false
}

func pollsIn(info *types.Info, root ast.Node, receives map[*ast.UnaryExpr]bool, clockGuards map[ast.Node]bool) bool {
	if root == nil {
		return false
	}
	found := false
	ast.PreorderStack(root, nil, func(node ast.Node, parents []ast.Node) bool {
		if found {
			return false
		}
		switch expr := node.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return false
		case *ast.FuncLit:
			return invokesLiteral(expr, parents)
		case *ast.UnaryExpr:
			found = receives[expr]
		case *ast.ReturnStmt:
			found = guardedByClock(parents, clockGuards) && !slices.ContainsFunc(parents, isFunctionLiteral)
		case *ast.BranchStmt:
			found = expr.Tok == token.BREAK && guardedByClock(parents, clockGuards) && breaksLoop(info, expr, root, parents)
		case *ast.ExprStmt:
			found = endsTest(info, expr.X) && guardedByClock(parents, clockGuards)
		}
		return !found
	})
	return found
}

func invokesLiteral(literal *ast.FuncLit, parents []ast.Node) bool {
	// Skip parentheses to check whether the literal is the callee.
	for i := len(parents) - 1; i >= 0; i-- {
		if _, paren := parents[i].(*ast.ParenExpr); paren {
			continue
		}
		call, ok := parents[i].(*ast.CallExpr)
		if !ok {
			break
		}
		var enclosing ast.Node
		if i > 0 {
			enclosing = parents[i-1]
		}
		return immediateLiteral(call, enclosing) == literal
	}
	return false
}

func isFunctionLiteral(node ast.Node) bool {
	_, ok := node.(*ast.FuncLit)
	return ok
}

func endsTest(info *types.Info, expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	return ok && testingMethod(info, call, "Fatal", "Fatalf", "FailNow", "Skip", "Skipf", "SkipNow")
}

func guardedByClock(parents []ast.Node, clockGuards map[ast.Node]bool) bool {
	for _, node := range parents {
		if clockGuards[node] {
			return true
		}
	}
	return false
}

func breaksLoop(info *types.Info, branch *ast.BranchStmt, root ast.Node, parents []ast.Node) bool {
	if branch.Label != nil {
		// A break to a label outside the body also leaves this loop.
		return info.Uses[branch.Label].Pos() < root.Pos()
	}
	for _, node := range parents {
		switch node.(type) {
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		}
	}
	return true
}
