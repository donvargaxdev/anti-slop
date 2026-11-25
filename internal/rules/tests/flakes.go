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
	flow   flakeFlow
	result *types.Var
}

type outcomePolarity uint8

const (
	failsWhenTrue outcomePolarity = 1 << iota
	failsWhenFalse
	shortMode
	refreshedError
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
				if c.returns.result != nil && len(stmt.Results) == 1 && c.outcomes != nil {
					c.storeObjectOutcome(c.returns.result, c.booleanOutcome(stmt.Results[0]))
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
			body.walk(stmt.Body, visit)
			return false
		case *ast.CallExpr:
			if literal := immediateLiteral(stmt, parent); literal != nil {
				c.walkImmediateCall(stmt, literal, visit)
				return false
			}
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
				c.storeOutcome(name, outcomes[i])
			}
			return false
		}
		return true
	})
}

func (c *flakeChecks) walkImmediateCall(call *ast.CallExpr, literal *ast.FuncLit, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	outcomes := c.assignmentOutcomes(call.Args, len(call.Args), visit)
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
			body.outcomes[params.At(i)] = outcomes[i]
		}
	}
	var returns flakeReturns
	body.returns = &returns
	if booleanResult(c.info, call) {
		returns.result = signature.Results().At(0)
	}
	body.walk(literal.Body, visit)
	if returns.result != nil {
		c.callOutcomes[call] = returns.flow.outcomes[returns.result]
		delete(returns.flow.outcomes, returns.result)
	}
	// Explicit returns and fallthrough both resume the caller. A testing
	// termination inside the literal may belong to a different test.
	if outcomes := mergeOutcomes(returns.flow.outcomes, body.outcomes); outcomes != nil {
		for param := range params.Variables() {
			delete(outcomes, param)
		}
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
	for count := int64(1); ; count++ {
		iteration := entry.fork()
		iteration.walk(condition, visit)
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
			// Both normal exits require a false condition. Break bypasses it.
			iteration.constrainOutcome(condition, failsWhenFalse)
			exits := iteration.flakeFlow
			if bounds.maySkip {
				c.constrainOutcome(condition, failsWhenFalse)
				exits = mergeFlakeFlows(exits, c.flakeFlow)
			}
			c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
			return
		}
		entry.flakeFlow = next
	}
}

func (c *flakeChecks) walkCommunications(body *ast.BlockStmt, label types.Object, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	entry := c.withTarget(label, false)
	var exits flakeFlow
	for _, clause := range body.List {
		branch := entry.fork()
		branch.walk(clause, visit)
		exits = mergeFlakeFlows(exits, branch.flakeFlow)
	}
	c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
}

type switchPath struct {
	flow    flakeFlow
	guarded bool
}

func (c *flakeChecks) walkCases(body *ast.BlockStmt, label types.Object, tag ast.Expr, visit func(flakeChecks, ast.Node, ast.Node) bool) {
	entry := c.withTarget(label, false)
	matches, unmatched := entry.matchSwitchCases(body, tag, visit)
	// Default is selected after all expressions miss, but fallthrough follows body order.
	var exits, fallingThrough flakeFlow
	fallthroughGuard := false
	hasDefault := false
	for _, stmt := range body.List {
		clause := stmt.(*ast.CaseClause)
		path := matches[clause]
		if clause.List == nil {
			hasDefault = true
			path = unmatched
		}
		branch := *entry
		branch.flakeFlow = path.flow
		guardedOnSuccess := path.guarded && (fallingThrough.outcomes == nil || fallthroughGuard)
		if c.retry == nil {
			branch.failureGuard = entry.failureGuard || path.guarded
		} else {
			branch.successCase = entry.successCase || guardedOnSuccess
		}
		// Fallthrough enters the next body without evaluating the case expressions.
		branch.flakeFlow = mergeFlakeFlows(branch.flakeFlow, fallingThrough)
		for _, stmt := range clause.Body {
			branch.walk(stmt, visit)
		}
		if len(clause.Body) > 0 && isFallthrough(clause.Body[len(clause.Body)-1]) {
			fallingThrough = branch.flakeFlow
			fallthroughGuard = guardedOnSuccess
		} else {
			fallingThrough = flakeFlow{}
			exits = mergeFlakeFlows(exits, branch.flakeFlow)
		}
	}
	if !hasDefault {
		unmatched.flow.failureGuard = entry.failureGuard || (c.retry == nil && unmatched.guarded)
		exits = mergeFlakeFlows(exits, unmatched.flow)
	}
	c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
}

func (c *flakeChecks) matchSwitchCases(body *ast.BlockStmt, tag ast.Expr, visit func(flakeChecks, ast.Node, ast.Node) bool) (map[*ast.CaseClause]switchPath, switchPath) {
	matches := make(map[*ast.CaseClause]switchPath)
	var attempt ast.Node
	if c.retry != nil {
		attempt = c.retry.body
	}
	unmatchedGuard := false
	for _, node := range body.List {
		clause := node.(*ast.CaseClause)
		if clause.List == nil {
			continue
		}
		// Later expressions are evaluated only after all earlier ones miss.
		matched := switchPath{guarded: true}
		for _, expr := range clause.List {
			c.walk(expr, visit)
			branch := c.fork()
			matchGuard := branch.constrainSwitchCase(tag, expr, true, attempt)
			missGuard := c.constrainSwitchCase(tag, expr, false, attempt)
			if c.retry != nil {
				// If one side proves success, the other can carry a failed attempt.
				branch.retryFailure = branch.retryFailure || missGuard
				c.retryFailure = c.retryFailure || matchGuard
			}
			matched.guarded = matched.guarded && (unmatchedGuard || matchGuard)
			matched.flow = mergeFlakeFlows(matched.flow, branch.flakeFlow)
			unmatchedGuard = unmatchedGuard || missGuard
		}
		matches[clause] = matched
	}
	return matches, switchPath{flow: c.flakeFlow, guarded: unmatchedGuard}
}

func (c *flakeChecks) constrainSwitchCase(tag, expr ast.Expr, matches bool, attempt ast.Node) bool {
	condition, trueOnMatch := c.switchCondition(tag, expr)
	if condition == nil {
		// Matching nil proves success. Missing nil proves failure.
		return matches == (attempt != nil) && c.errorComparedWithNil(tag, expr, attempt)
	}
	truth := matches == trueOnMatch
	guarded := c.failureCondition(condition, truth == (attempt == nil), attempt)
	possible := failsWhenFalse
	if truth {
		possible = failsWhenTrue
	}
	c.constrainOutcome(condition, possible)
	return guarded
}

// switchCondition finds the boolean operand and its value when the case matches.
func (c flakeChecks) switchCondition(tag, expr ast.Expr) (ast.Expr, bool) {
	if tag == nil {
		return expr, true
	}
	if value := c.info.Types[tag].Value; value != nil && value.Kind() == constant.Bool {
		return expr, constant.BoolVal(value)
	}
	value := c.info.Types[expr].Value
	if value != nil && value.Kind() == constant.Bool {
		return tag, constant.BoolVal(value)
	}
	return nil, false
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
	// The exemption applies only when every incoming path carries short mode.
	for object, polarity := range dst {
		if polarity == shortMode && src[object] != shortMode {
			delete(dst, object)
		}
	}
	for object, polarity := range src {
		if polarity != shortMode {
			dst[object] |= polarity
		}
	}
	return dst
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
			if c.retryHeader {
				outcomes[i] = c.retryErrorOutcome(value)
			}
			continue
		}
		outcomes[i] = c.booleanOutcome(value)
		if c.retryHeader && outcomes[i] == 0 && c.isCheckCall(value) {
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
func (c flakeChecks) constrainOutcome(condition ast.Expr, possible outcomePolarity) {
	switch expr := ast.Unparen(condition).(type) {
	case *ast.Ident:
		polarity := c.outcomes[c.info.Uses[expr]]
		if polarity != shortMode {
			c.storeOutcome(expr, polarity&possible)
		}
	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			c.constrainOutcome(expr.X, possible^(failsWhenTrue|failsWhenFalse))
		}
	case *ast.BinaryExpr:
		if operand, same := booleanComparison(c.info, expr); operand != nil {
			if !same {
				possible ^= failsWhenTrue | failsWhenFalse
			}
			c.constrainOutcome(operand, possible)
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
	if polarity == 0 {
		delete(c.outcomes, object)
	} else {
		c.outcomes[object] = polarity
	}
}
