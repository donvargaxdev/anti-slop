package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"
)

var Flakes = &analysis.Analyzer{
	Name: "tests_flakes",
	Doc:  "report retries and failure-dependent skips in Go test code",
	Run:  runFlakes,
}

type flakeChecks struct {
	info *types.Info
	flakeFlow
	target       *branchTarget[flakeFlow]
	returns      *flakeReturns
	callOutcomes map[*ast.CallExpr]outcomePolarity
	sharedErrors map[types.Object]bool
	mutableVars  map[types.Object]bool
	retry        *retryScan
	successCase  bool
	retryHeader  bool
}

type flakeFlow struct {
	// A nil map marks a path that jumped away.
	outcomes     map[types.Object]outcomePolarity
	failureGuard bool
	// A failed check can reach this point during the retry scan.
	retryFailure bool
}

type flakeReturns struct {
	flow      flakeFlow
	result    *types.Var
	guarded   outcomePolarity
	unguarded outcomePolarity
	onlyShort bool
}

type outcomePolarity uint16

const (
	failsWhenTrue outcomePolarity = 1 << iota
	failsWhenFalse
	shortMode
	refreshedError
	nilError
	constantTrue
	constantFalse
	guardedTrue
	guardedFalse
)

func runFlakes(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	checks := flakeChecks{
		info:         info,
		flakeFlow:    flakeFlow{outcomes: make(map[types.Object]outcomePolarity)},
		callOutcomes: make(map[*ast.CallExpr]outcomePolarity),
	}
	reported := make(map[token.Pos]bool)
	reportOnce := func(node ast.Node, reason string) {
		if !reported[node.Pos()] {
			report(pass, node, reason)
			reported[node.Pos()] = true
		}
	}
	visitTestFiles(pass, func(file *ast.File) {
		checks.sharedErrors = sharedErrorVariables(info, file)
		checks.mutableVars = mutableVariables(info, file)
		checks.walk(file, func(checks flakeChecks, node, parent ast.Node) bool {
			checks.reportFinding(node, parent, reportOnce)
			return true
		})
	})
	return nil, nil
}

func (c flakeChecks) reportFinding(node, parent ast.Node, reportOnce func(ast.Node, string)) {
	switch stmt := node.(type) {
	case *ast.ForStmt:
		if forBounds(c.info, stmt.Cond).maxIterations != 0 && (c.retriesInHeader(stmt, parent) || c.retriesLoop(stmt.Body, stmt.Post, parent)) {
			reportOnce(stmt, "retrying a check or test body")
		}
	case *ast.RangeStmt:
		maxIterations := rangeBounds(c.info, stmt.X).maxIterations
		if (maxIterations == unknownIterations || maxIterations > 1) && c.retriesLoop(stmt.Body, nil, parent) {
			reportOnce(stmt, "retrying a check or test body")
		}
	case *ast.CallExpr:
		if c.failureGuard && testingMethod(c.info, stmt, "Skip", "Skipf", "SkipNow") {
			reportOnce(stmt, "skipping after a failure")
		}
		if wrapsTestInRetry(c.info, stmt) {
			reportOnce(stmt, "retrying a test body")
		}
	}
}

// walk tracks stored outcomes in source order and joins alternative branches.
func (c *flakeChecks) walk(root ast.Node, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	if root == nil {
		return
	}
	ast.PreorderStack(root, nil, func(node ast.Node, parents []ast.Node) bool {
		if c.outcomes == nil {
			return false
		}
		var parent ast.Node
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		}
		if stmt, ok := node.(*ast.IfStmt); ok {
			c.walk(stmt.Init, visit)
			if visit(*c, stmt, parent) {
				c.walk(stmt.Cond, visit)
				then := c.fork()
				otherwise := c.fork()
				if c.retry != nil {
					c.trackRetryBranches(stmt, parent, then, otherwise)
				}
				then.failureGuard = c.failureGuard || c.failureCondition(stmt.Cond, true, nil)
				then.constrainOutcome(stmt.Cond, failsWhenTrue)
				then.walk(stmt.Body, visit)
				otherwise.failureGuard = c.failureGuard || c.failureCondition(stmt.Cond, false, nil)
				otherwise.constrainOutcome(stmt.Cond, failsWhenFalse)
				otherwise.walk(stmt.Else, visit)
				c.flakeFlow = mergeFlakeFlows(otherwise.flakeFlow, then.flakeFlow)
			}
			return false
		}
		if !visit(*c, node, parent) {
			return false
		}
		var label types.Object
		if labeled, ok := parent.(*ast.LabeledStmt); ok {
			label = c.info.Defs[labeled.Label]
		}
		switch stmt := node.(type) {
		case *ast.ForStmt:
			c.walk(stmt.Init, visit)
			c.walkLoop(label, stmt.Body, stmt.Cond, stmt.Post, forBounds(c.info, stmt.Cond), visit, nil)
			return false
		case *ast.RangeStmt:
			c.walk(stmt.X, visit)
			c.walkLoop(label, stmt.Body, nil, nil, rangeBounds(c.info, stmt.X), visit, func(iteration *flakeChecks) {
				for _, target := range [...]ast.Expr{stmt.Key, stmt.Value} {
					iteration.walk(target, visit)
					if name, ok := ast.Unparen(target).(*ast.Ident); ok {
						iteration.storeOutcome(name, 0)
					}
				}
			})
			return false
		case *ast.SwitchStmt:
			c.walk(stmt.Init, visit)
			c.walk(stmt.Tag, visit)
			c.walkCases(stmt.Body, label, stmt.Tag, visit)
			return false
		case *ast.TypeSwitchStmt:
			c.walk(stmt.Init, visit)
			c.walk(stmt.Assign, visit)
			c.walkCases(stmt.Body, label, nil, visit)
			return false
		case *ast.SelectStmt:
			c.walkCommunications(stmt.Body, label, visit)
			return false
		case *ast.ReturnStmt:
			for _, result := range stmt.Results {
				c.walk(result, visit)
			}
			if c.returns != nil {
				if c.returns.result != nil && c.outcomes != nil {
					c.recordBooleanReturn(stmt)
				}
				c.returns.flow = mergeFlakeFlows(c.returns.flow, c.flakeFlow)
			}
			c.outcomes = nil
			return false
		case *ast.ExprStmt:
			if endsTest(c.info, stmt.X) {
				c.walk(stmt.X, visit)
				c.outcomes = nil
				return false
			}
		case *ast.BranchStmt:
			if stmt.Tok == token.BREAK || stmt.Tok == token.CONTINUE {
				c.recordBranch(stmt)
			}
		case *ast.FuncDecl:
			if stmt.Body != nil {
				c.fork().walk(stmt.Body, visit)
			}
			return false
		case *ast.FuncLit:
			body := c.fork()
			body.failureGuard = false
			body.returns = nil
			// A delayed closure can observe later writes to its captures.
			for object := range body.outcomes {
				if body.mutableVars[object] {
					delete(body.outcomes, object)
				}
			}
			body.walk(stmt.Body, visit)
			return false
		case *ast.CallExpr:
			if literal := immediateLiteral(stmt, parent); literal != nil {
				c.walkImmediateCall(stmt, literal, visit)
			} else {
				c.walkCall(stmt, parent, visit)
			}
			return false
		case *ast.BinaryExpr:
			if stmt.Op == token.LAND || stmt.Op == token.LOR {
				c.walkShortCircuit(stmt, visit)
				return false
			}
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				c.walk(lhs, visit)
			}
			outcomes := c.assignmentOutcomes(stmt.Rhs, len(stmt.Lhs), visit)
			for i, lhs := range stmt.Lhs {
				if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					c.storeOutcome(ident, outcomes[i])
				}
			}
			return false
		case *ast.ValueSpec:
			outcomes := c.assignmentOutcomes(stmt.Values, len(stmt.Names), visit)
			for i, name := range stmt.Names {
				if len(stmt.Values) == 0 {
					outcomes[i] = nilError
					if booleanResult(c.info, name) {
						outcomes[i] = c.constantAssignment(false)
					}
				}
				c.storeOutcome(name, outcomes[i])
			}
			return false
		}
		return true
	})
}

func (c *flakeChecks) walkCall(call *ast.CallExpr, parent ast.Node, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	c.walk(call.Fun, visit)
	for _, arg := range call.Args {
		c.walk(arg, visit)
	}
	forgetCallWrites(c.info, call, parent, c.outcomes)
}

func (c *flakeChecks) walkImmediateCall(call *ast.CallExpr, literal *ast.FuncLit, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	outcomes := c.assignmentOutcomes(call.Args, len(call.Args), visit)
	forgetCallWrites(c.info, call, nil, c.outcomes)
	if !visit(*c, literal, call) {
		return
	}
	body := c.fork()
	signature := c.info.TypeOf(literal).(*types.Signature)
	params := signature.Params()
	fixed := params.Len()
	if signature.Variadic() {
		// Variadic arguments populate a slice rather than a scalar outcome.
		fixed--
	}
	for i := range min(fixed, len(outcomes)) {
		if outcomes[i] != 0 {
			body.storeObjectOutcome(params.At(i), outcomes[i])
		}
	}
	var returns flakeReturns
	body.returns = &returns
	results := signature.Results()
	for result := range results.Variables() {
		if result.Name() != "" {
			body.storeObjectOutcome(result, nilError)
		}
	}
	if booleanResult(c.info, call) {
		returns.result = results.At(0)
		if returns.result.Name() != "" {
			body.storeObjectOutcome(returns.result, constantFalse)
		}
	}
	body.walk(literal.Body, visit)
	if returns.result != nil {
		if deferMayWriteResult(c.info, literal.Body, returns.result) {
			delete(c.callOutcomes, call)
		} else {
			c.callOutcomes[call] = returns.booleanOutcome()
		}
	}
	// Explicit returns and fallthrough both resume the caller. A testing
	// termination inside the literal may belong to a different test.
	if outcomes := mergeOutcomes(returns.flow.outcomes, body.outcomes); outcomes != nil {
		for param := range params.Variables() {
			delete(outcomes, param)
		}
		for result := range results.Variables() {
			delete(outcomes, result)
		}
		forgetCallWrites(c.info, call, nil, outcomes)
		c.outcomes = outcomes
	}
}

func (c *flakeChecks) walkShortCircuit(expr *ast.BinaryExpr, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	c.walk(expr.X, visit)
	if value := c.info.Types[expr.X].Value; value != nil {
		if constant.BoolVal(value) == (expr.Op == token.LAND) {
			c.walk(expr.Y, visit)
		}
		return
	}
	right := c.fork()
	evaluate, skip := failsWhenTrue, failsWhenFalse
	if expr.Op == token.LOR {
		evaluate, skip = skip, evaluate
	}
	right.failureGuard = c.failureGuard || c.failureCondition(expr.X, evaluate == failsWhenTrue, nil)
	right.constrainOutcome(expr.X, evaluate)
	c.constrainOutcome(expr.X, skip)
	right.walk(expr.Y, visit)
	c.flakeFlow = mergeFlakeFlows(c.flakeFlow, right.flakeFlow)
}

func (c flakeChecks) fork() *flakeChecks {
	c.outcomes = maps.Clone(c.outcomes)
	return &c
}

func (c flakeChecks) withTarget(label types.Object, isLoop bool) *flakeChecks {
	branch := c.fork()
	branch.target = &branchTarget[flakeFlow]{label: label, isLoop: isLoop, parent: c.target}
	return branch
}

func (c *flakeChecks) recordBranch(stmt *ast.BranchStmt) {
	target := c.target.destination(c.info, stmt)
	if target == nil {
		if c.retry != nil {
			c.outcomes = nil
		}
		return
	}
	if stmt.Tok == token.CONTINUE {
		target.continues = mergeFlakeFlows(target.continues, c.flakeFlow)
	} else {
		target.breaks = mergeFlakeFlows(target.breaks, c.flakeFlow)
	}
	c.outcomes = nil
}

func (c *flakeChecks) walkLoop(label types.Object, body *ast.BlockStmt, condition ast.Expr, post ast.Stmt, bounds loopBounds, visit func(flakeChecks, ast.Node, ast.Node) bool, beforeBody func(*flakeChecks)) {
	if bounds.maxIterations == 0 {
		return
	}
	entry := c.withTarget(label, true)
	var skipped flakeFlow
	for count := int64(1); ; count++ {
		iteration := entry.fork()
		iteration.walk(condition, visit)
		if count == 1 && bounds.maySkip {
			skipped = iteration.fork().exitOnFalse(condition)
		}
		iteration.failureGuard = iteration.failureGuard || iteration.failureCondition(condition, true, nil)
		iteration.constrainOutcome(condition, failsWhenTrue)
		if beforeBody != nil {
			beforeBody(iteration)
		}
		iteration.walk(body, visit)
		// Continue runs the post statement. Break reaches the next statement directly.
		iteration.flakeFlow = mergeFlakeFlows(iteration.flakeFlow, entry.target.continues)
		iteration.walk(post, visit)
		// The next iteration can also start with values from the first entry.
		next := mergeFlakeFlows(entry.fork().flakeFlow, iteration.flakeFlow)
		if count == bounds.maxIterations || maps.Equal(entry.outcomes, next.outcomes) {
			// A normal exit tests the condition again after the post statement.
			iteration.walk(condition, visit)
			exits := mergeFlakeFlows(iteration.exitOnFalse(condition), skipped)
			c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
			return
		}
		entry.flakeFlow = next
	}
}

func (c *flakeChecks) exitOnFalse(condition ast.Expr) flakeFlow {
	c.failureGuard = c.failureGuard || c.failureCondition(condition, false, nil)
	c.constrainOutcome(condition, failsWhenFalse)
	return c.flakeFlow
}

func (c *flakeChecks) walkCommunications(body *ast.BlockStmt, label types.Object, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	entry := c.withTarget(label, false)
	visitSelectOperands(body, func(expr ast.Expr) { entry.walk(expr, visit) })
	var exits flakeFlow
	for _, stmt := range body.List {
		clause := stmt.(*ast.CommClause)
		branch := entry.fork()
		visitReceiveTargets(clause.Comm, func(expr ast.Expr) { branch.walk(expr, visit) }, func(name *ast.Ident) {
			branch.storeOutcome(name, 0)
		})
		for _, bodyStmt := range clause.Body {
			branch.walk(bodyStmt, visit)
		}
		exits = mergeFlakeFlows(exits, branch.flakeFlow)
	}
	c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
}

func mergeFlakeFlows(dst, src flakeFlow) flakeFlow {
	// Every path that reaches the join must require failure.
	guarded := (dst.outcomes == nil || dst.failureGuard) && (src.outcomes == nil || src.failureGuard)
	retryFailure := (dst.outcomes != nil && dst.retryFailure) || (src.outcomes != nil && src.retryFailure)
	return flakeFlow{outcomes: mergeOutcomes(dst.outcomes, src.outcomes), failureGuard: guarded, retryFailure: retryFailure}
}

func mergeOutcomes(dst, src map[types.Object]outcomePolarity) map[types.Object]outcomePolarity {
	if dst == nil {
		return maps.Clone(src)
	}
	if src == nil {
		return dst
	}
	for object, polarity := range dst {
		if _, shared := src[object]; !shared {
			writeOutcome(dst, object, joinOutcome(polarity, 0))
		}
	}
	for object, polarity := range src {
		writeOutcome(dst, object, joinOutcome(dst[object], polarity))
	}
	return dst
}

func writeOutcome(outcomes map[types.Object]outcomePolarity, object types.Object, polarity outcomePolarity) {
	if polarity == 0 {
		delete(outcomes, object)
	} else {
		outcomes[object] = polarity
	}
}

// assignmentOutcomes evaluates each operand before any target is updated.
func (c *flakeChecks) assignmentOutcomes(values []ast.Expr, targetCount int, visit func(flakeChecks, ast.Node, ast.Node) bool) []outcomePolarity {
	outcomes := make([]outcomePolarity, targetCount)
	if len(values) != targetCount {
		for _, value := range values {
			c.walk(value, visit)
		}
		if c.retryHeader {
			c.retryTupleOutcomes(values, outcomes)
		}
		return outcomes
	}
	for i, value := range values {
		c.walk(value, visit)
		if !booleanResult(c.info, value) {
			if c.isNil(value) || c.knownNilError(value) {
				outcomes[i] = nilError
			} else if c.retryHeader {
				outcomes[i] = c.retryErrorOutcome(value)
			}
			continue
		}
		if constantValue := c.info.Types[value].Value; constantValue != nil {
			outcomes[i] = c.constantAssignment(constant.BoolVal(constantValue))
			continue
		}
		if c.failureGuard && c.retry == nil && !c.retryHeader {
			// Either value requires failure on this path. Retry scans need
			// the outcome of the current attempt, which can differ.
			outcomes[i] = guardedTrue | guardedFalse
			continue
		}
		outcomes[i] = c.booleanOutcome(value)
		if c.retryHeader && outcomes[i] == 0 && c.retryBooleanCheck(value) {
			outcomes[i] = failsWhenFalse
		}
	}
	return outcomes
}

func (c flakeChecks) booleanOutcome(value ast.Expr) outcomePolarity {
	if c.isShortMode(value) {
		return shortMode
	}
	var polarity outcomePolarity
	if c.failureCondition(value, true, nil) {
		polarity |= failsWhenTrue
	}
	if c.failureCondition(value, false, nil) {
		polarity |= failsWhenFalse
	}
	return polarity
}

func (c flakeChecks) isShortMode(value ast.Expr) bool {
	switch expr := ast.Unparen(value).(type) {
	case *ast.CallExpr:
		return c.callOutcomes[expr] == shortMode || packageCall(c.info, expr, "testing", "Short")
	case *ast.Ident:
		return c.outcomes[c.info.ObjectOf(expr)] == shortMode
	case *ast.UnaryExpr:
		return expr.Op == token.NOT && c.isShortMode(expr.X)
	case *ast.BinaryExpr:
		operand, _ := booleanComparison(c.info, expr)
		return c.isShortMode(operand)
	}
	return false
}

// A branch testing a stored outcome can only carry its matching polarity.
func (c *flakeChecks) constrainOutcome(condition ast.Expr, possible outcomePolarity, later ...ast.Expr) {
	if c.outcomes == nil {
		return
	}
	switch expr := ast.Unparen(condition).(type) {
	case *ast.Ident:
		object := c.info.Uses[expr]
		polarity := c.outcomes[object]
		if polarity != shortMode && booleanResult(c.info, expr) && !c.outcomeReassigned(object, later) {
			c.storeOutcome(expr, restrictBooleanOutcome(polarity, possible))
		}
	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			c.constrainOutcome(expr.X, possible^(failsWhenTrue|failsWhenFalse), later...)
		}
	case *ast.CallExpr:
		if c.isErrorsIsNil(expr) {
			c.constrainError(expr.Args[0], possible == failsWhenTrue)
		}
	case *ast.BinaryExpr:
		if (expr.Op == token.LAND && possible == failsWhenTrue) ||
			(expr.Op == token.LOR && possible == failsWhenFalse) {
			c.constrainOutcome(expr.X, possible, append(later, expr.Y)...)
			c.constrainOutcome(expr.Y, possible, later...)
		} else if operand, same := booleanComparison(c.info, expr); operand != nil {
			if !same {
				possible ^= failsWhenTrue | failsWhenFalse
			}
			c.constrainOutcome(operand, possible, later...)
		} else if expr.Op == token.EQL || expr.Op == token.NEQ {
			equal := (expr.Op == token.EQL) == (possible == failsWhenTrue)
			if c.isNil(expr.Y) {
				c.constrainError(expr.X, equal)
			} else if c.isNil(expr.X) {
				c.constrainError(expr.Y, equal)
			}
		}
	}
}

func (c flakeChecks) storeOutcome(name *ast.Ident, polarity outcomePolarity) {
	if name.Name == "_" {
		return
	}
	c.storeObjectOutcome(c.info.ObjectOf(name), polarity)
}

func (c flakeChecks) storeObjectOutcome(object types.Object, polarity outcomePolarity) {
	if polarity == nilError && !c.tracksNilError(object) {
		polarity = 0
	}
	writeOutcome(c.outcomes, object, polarity)
}
