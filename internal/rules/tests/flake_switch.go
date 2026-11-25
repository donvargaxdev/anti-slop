package tests

import (
	"go/ast"
	"go/constant"
	"go/types"
)

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
	// The tag is evaluated once, before any case expression can change its operands.
	tagState := c.fork()
	var evaluated []ast.Expr
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
			evaluated = append(evaluated, expr)
			branch := c.fork()
			matchGuard := branch.constrainSwitchCase(tag, expr, true, attempt, tagState, evaluated)
			missGuard := c.constrainSwitchCase(tag, expr, false, attempt, tagState, evaluated)
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

func (c *flakeChecks) constrainSwitchCase(tag, expr ast.Expr, matches bool, attempt ast.Node, tagState *flakeChecks, evaluated []ast.Expr) bool {
	condition, trueOnMatch := c.switchCondition(tag, expr)
	if condition == nil {
		// Matching nil proves success. Missing nil proves failure.
		guarded := matches == (attempt != nil) && tagState.errorComparedWithNil(tag, expr, attempt)
		if c.isNil(expr) {
			c.constrainError(tag, matches)
		}
		return guarded
	}
	truth := matches == trueOnMatch
	guardState := c
	var later []ast.Expr
	if condition == tag {
		guardState = tagState
		// Only operands that still hold their original values can be narrowed.
		later = evaluated
	}
	guarded := guardState.failureCondition(condition, truth == (attempt == nil), attempt)
	possible := failsWhenFalse
	if truth {
		possible = failsWhenTrue
	}
	c.constrainOutcome(condition, possible, later...)
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
