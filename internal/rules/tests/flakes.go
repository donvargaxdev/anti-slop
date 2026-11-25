package tests

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"strings"

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
	target      *branchTarget[flakeFlow]
	retryBody   *ast.BlockStmt
	successCase bool
	retryHeader bool
}

type flakeFlow struct {
	// A nil map marks a path that jumped away.
	outcomes     map[types.Object]outcomePolarity
	failureGuard bool
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
	checks := flakeChecks{info: info, flakeFlow: flakeFlow{outcomes: make(map[types.Object]outcomePolarity)}}
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
		if forBounds(c.info, stmt.Cond).maxIterations != 0 && (c.retriesInHeader(stmt, parent) || c.retriesLoop(stmt.Body)) {
			reportOnce(stmt, "retrying a check or test body")
		}
	case *ast.RangeStmt:
		maxIterations := rangeBounds(c.info, stmt.X).maxIterations
		if (maxIterations == unknownIterations || maxIterations > 1) && c.retriesLoop(stmt.Body) {
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
				then.failureGuard = c.failureGuard || c.failureCondition(stmt.Cond, true, nil)
				then.constrainOutcome(stmt.Cond, failsWhenTrue)
				then.walk(stmt.Body, visit)
				otherwise := c.fork()
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
			c.walkLoop(label, stmt.Body, stmt.Post, forBounds(c.info, stmt.Cond), visit, func(iteration *flakeChecks) {
				iteration.walk(stmt.Cond, visit)
			})
			return false
		case *ast.RangeStmt:
			c.walk(stmt.X, visit)
			c.walkLoop(label, stmt.Body, nil, rangeBounds(c.info, stmt.X), visit, func(iteration *flakeChecks) {
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
			if !calledImmediately(stmt, parents) {
				body.failureGuard = false
			}
			body.walk(stmt.Body, visit)
			return false
		case *ast.AssignStmt:
			outcomes := c.assignmentOutcomes(stmt.Rhs, len(stmt.Lhs))
			for i, lhs := range stmt.Lhs {
				if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					c.storeOutcome(ident, outcomes[i])
				}
			}
		case *ast.ValueSpec:
			outcomes := c.assignmentOutcomes(stmt.Values, len(stmt.Names))
			for i, name := range stmt.Names {
				c.storeOutcome(name, outcomes[i])
			}
		}
		return true
	})
}

func calledImmediately(literal *ast.FuncLit, parents []ast.Node) bool {
	for i := len(parents) - 1; i >= 0; i-- {
		if _, ok := parents[i].(*ast.ParenExpr); ok {
			continue
		}
		call, ok := parents[i].(*ast.CallExpr)
		if !ok || ast.Unparen(call.Fun) != literal {
			return false
		}
		// Only the outer call is delayed. Its arguments are evaluated now.
		if i > 0 {
			switch parents[i-1].(type) {
			case *ast.DeferStmt, *ast.GoStmt:
				return false
			}
		}
		return true
	}
	return false
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
		return
	}
	if stmt.Tok == token.CONTINUE {
		target.continues = mergeFlakeFlows(target.continues, c.flakeFlow)
	} else {
		target.breaks = mergeFlakeFlows(target.breaks, c.flakeFlow)
	}
	c.outcomes = nil
}

func (c *flakeChecks) walkLoop(label types.Object, body *ast.BlockStmt, post ast.Stmt, bounds loopBounds, visit func(flakeChecks, ast.Node, ast.Node) bool, beforeBody func(*flakeChecks)) {
	if bounds.maxIterations == 0 {
		return
	}
	entry := c.withTarget(label, true)
	for count := int64(1); ; count++ {
		iteration := entry.fork()
		beforeBody(iteration)
		iteration.walk(body, visit)
		// Continue runs the post statement. Break reaches the next statement directly.
		iteration.flakeFlow = mergeFlakeFlows(iteration.flakeFlow, entry.target.continues)
		iteration.walk(post, visit)
		// The next iteration can also start with values from the first entry.
		next := mergeFlakeFlows(entry.fork().flakeFlow, iteration.flakeFlow)
		if count == bounds.maxIterations || maps.Equal(entry.outcomes, next.outcomes) {
			exits := mergeFlakeFlows(iteration.flakeFlow, entry.target.breaks)
			if bounds.maySkip {
				exits = mergeFlakeFlows(exits, c.flakeFlow)
			}
			c.flakeFlow = exits
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
	outcomes map[types.Object]outcomePolarity
	guarded  bool
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
		branch.outcomes = path.outcomes
		guardedOnSuccess := path.guarded && (fallingThrough.outcomes == nil || fallthroughGuard)
		if c.retryBody == nil {
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
		exits = mergeFlakeFlows(exits, flakeFlow{
			outcomes:     unmatched.outcomes,
			failureGuard: entry.failureGuard || (c.retryBody == nil && unmatched.guarded),
		})
	}
	c.flakeFlow = mergeFlakeFlows(exits, entry.target.breaks)
}

func (c *flakeChecks) matchSwitchCases(body *ast.BlockStmt, tag ast.Expr, visit func(flakeChecks, ast.Node, ast.Node) bool) (map[*ast.CaseClause]switchPath, switchPath) {
	matches := make(map[*ast.CaseClause]switchPath)
	var attempt ast.Node
	if c.retryBody != nil {
		attempt = c.retryBody
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
			matched.guarded = matched.guarded && (unmatchedGuard || matchGuard)
			matched.outcomes = mergeOutcomes(matched.outcomes, branch.outcomes)

			missGuard := c.constrainSwitchCase(tag, expr, false, attempt)
			unmatchedGuard = unmatchedGuard || missGuard
		}
		matches[clause] = matched
	}
	return matches, switchPath{outcomes: c.outcomes, guarded: unmatchedGuard}
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
	return flakeFlow{outcomes: mergeOutcomes(dst.outcomes, src.outcomes), failureGuard: guarded}
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

// assignmentOutcomes reads the incoming state before any target is updated.
func (c flakeChecks) assignmentOutcomes(values []ast.Expr, targetCount int) []outcomePolarity {
	outcomes := make([]outcomePolarity, targetCount)
	if len(values) != targetCount {
		if c.retryHeader {
			c.retryTupleOutcomes(values, outcomes)
		}
		return outcomes
	}
	for i, value := range values {
		if !booleanResult(c.info, value) {
			if c.retryHeader {
				outcomes[i] = c.retryErrorOutcome(value)
			}
			continue
		}
		if c.isShortMode(value) {
			outcomes[i] = shortMode
			continue
		}
		if c.failureCondition(value, true, nil) {
			outcomes[i] |= failsWhenTrue
		}
		if c.failureCondition(value, false, nil) {
			outcomes[i] |= failsWhenFalse
		}
		if c.retryHeader && outcomes[i] == 0 && c.isCheckCall(value) {
			outcomes[i] = failsWhenFalse
		}
	}
	return outcomes
}

func (c flakeChecks) isShortMode(value ast.Expr) bool {
	switch expr := ast.Unparen(value).(type) {
	case *ast.CallExpr:
		return packageCall(c.info, expr, "testing", "Short")
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
	object := c.info.ObjectOf(name)
	if polarity == 0 {
		delete(c.outcomes, object)
	} else {
		c.outcomes[object] = polarity
	}
}

func (c flakeChecks) retriesLoop(body *ast.BlockStmt) bool {
	// The main traversal still needs the state from before entering the loop.
	c.outcomes = maps.Clone(c.outcomes)
	// Only targets inside the attempt are tracked during this walk.
	c.target = nil
	c.retryBody = body
	found := false
	c.walk(body, func(c flakeChecks, node, parent ast.Node) bool {
		if found {
			return false
		}
		switch stmt := node.(type) {
		case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		case *ast.ReturnStmt, *ast.BranchStmt:
			found = c.successCase && c.exitsAttempt(stmt)
		case *ast.IfStmt:
			for _, branch := range c.outcomeBranches(stmt, false, body) {
				if c.exitsAttempt(lastStatement(branch)) {
					found = true
					return false
				}
			}
			found = c.continuesUntilSuccess(stmt, body, parent)
		}
		return !found
	})
	return found
}

func (c flakeChecks) outcomeBranches(stmt *ast.IfStmt, failure bool, attempt ast.Node) [2]ast.Stmt {
	var branches [2]ast.Stmt
	if c.failureCondition(stmt.Cond, failure, attempt) {
		branches[0] = stmt.Body
	}
	if stmt.Else != nil && c.failureCondition(stmt.Cond, !failure, attempt) {
		branches[1] = stmt.Else
	}
	return branches
}

func lastStatement(branch ast.Stmt) ast.Stmt {
	block, ok := branch.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return nil
	}
	return block.List[len(block.List)-1]
}

func (c flakeChecks) exitsAttempt(stmt ast.Node) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.BREAK && c.target.destination(c.info, stmt) == nil
	}
	return false
}

func (c flakeChecks) continuesUntilSuccess(stmt *ast.IfStmt, body *ast.BlockStmt, parent ast.Node) bool {
	if stmt.Else != nil || !c.failureCondition(stmt.Cond, true, body) {
		return false
	}
	branch, ok := lastStatement(stmt.Body).(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE {
		return false
	}
	// A continue on failure is retry evidence only if success leaves the loop.
	block, ok := parent.(*ast.BlockStmt)
	if !ok {
		return false
	}
	for _, sibling := range block.List {
		if sibling.Pos() < stmt.End() {
			continue
		}
		sibling = unlabel(sibling)
		if c.exitsAttempt(sibling) {
			return true
		}
		if _, jump := sibling.(*ast.BranchStmt); jump {
			return false
		}
	}
	return false
}

func wrapsTestInRetry(info *types.Info, call *ast.CallExpr) bool {
	fn := calledFunc(info, call)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	name := strings.ToLower(fn.Name())
	path := fn.Pkg().Path()
	if name != "retry" && name != "retrytest" && name != "retrysubtest" &&
		path != "retry" && !strings.HasSuffix(path, "/retry") {
		return false
	}
	for _, arg := range call.Args {
		if typ := info.TypeOf(arg); typ != nil {
			if sig, ok := typ.Underlying().(*types.Signature); ok && sig.Params().Len() > 0 &&
				isTestingT(sig.Params().At(0).Type()) {
				return true
			}
		}
		body, ok := ast.Unparen(arg).(*ast.FuncLit)
		if ok && contains(body.Body, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			return ok && isTestingT(info.TypeOf(ident))
		}) {
			return true
		}
	}
	return false
}

func isTestingT(typ types.Type) bool {
	pointer, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "T"
}
