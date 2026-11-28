package funclen

import (
	"go/ast"
	"go/token"
)

type tables map[*ast.CompositeLit]struct{}

func caseTables(file *ast.File) tables {
	found := make(tables)
	for node := range ast.Preorder(file) {
		switch node := node.(type) {
		case *ast.RangeStmt:
			if table := caseLiteral(node.X); table != nil {
				found[table] = struct{}{}
			}
		case *ast.BlockStmt:
			found.addAdjacent(node.List)
		case *ast.CaseClause:
			found.addAdjacent(node.Body)
		case *ast.CommClause:
			found.addAdjacent(node.Body)
		}
	}
	return found
}

func (found tables) addAdjacent(statements []ast.Stmt) {
	for i := 1; i < len(statements); i++ {
		previous := statements[i-1]
		current := unlabel(statements[i])
		if name, table := assignedLiteral(previous, token.DEFINE); table != nil && leadsToRange(current, name) {
			found[table] = struct{}{}
		}
		loop, ok := current.(*ast.RangeStmt)
		if !ok {
			continue
		}
		name := rangedName(loop.X)
		if i >= 2 && startsWithBreak(previous) {
			declared, table := assignedLiteral(statements[i-2], token.DEFINE)
			if table != nil && declared == name {
				found[table] = struct{}{}
			}
		}
		found.addConditionalAssignments(previous, name)
	}
}

func leadsToRange(statement ast.Stmt, name string) bool {
	pending := []ast.Stmt{statement}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		switch current := unlabel(current).(type) {
		case *ast.RangeStmt:
			if rangedName(current.X) == name {
				return true
			}
		case *ast.BlockStmt:
			if len(current.List) > 0 {
				pending = append(pending, current.List[0])
			}
		case *ast.IfStmt:
			if current.Init == nil {
				pending = append(pending, current.Body, current.Else)
			}
		case *ast.ForStmt:
			if current.Init == nil {
				pending = append(pending, current.Body)
			}
		case *ast.ExprStmt:
			if body := subtestBody(current.X, name); body != nil {
				pending = append(pending, body)
			}
		}
	}
	return false
}

func subtestBody(expression ast.Expr, name string) *ast.BlockStmt {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return nil
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "Run" {
		return nil
	}
	callback, ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
	if !ok {
		return nil
	}
	for _, field := range callback.Type.Params.List {
		for _, parameter := range field.Names {
			if parameter.Name == name {
				return nil
			}
		}
	}
	return callback.Body
}

func (found tables) addConditionalAssignments(statement ast.Stmt, name string) {
	conditional, ok := unlabel(statement).(*ast.IfStmt)
	if !ok || conditional.Init != nil {
		return
	}
	otherwise, _ := conditional.Else.(*ast.BlockStmt)
	for _, branch := range []*ast.BlockStmt{conditional.Body, otherwise} {
		if branch == nil || len(branch.List) != 1 {
			continue
		}
		assigned, table := assignedLiteral(branch.List[0], token.ASSIGN)
		if table != nil && assigned == name {
			found[table] = struct{}{}
		}
	}
}

func unlabel(statement ast.Stmt) ast.Stmt {
	for {
		label, ok := statement.(*ast.LabeledStmt)
		if !ok {
			return statement
		}
		statement = label.Stmt
	}
}

func rangedName(expression ast.Expr) string {
	if name, ok := ast.Unparen(expression).(*ast.Ident); ok {
		return name.Name
	}
	return ""
}

func assignedLiteral(statement ast.Stmt, operator token.Token) (string, *ast.CompositeLit) {
	assignment, ok := unlabel(statement).(*ast.AssignStmt)
	if !ok || assignment.Tok != operator {
		return "", nil
	}
	if len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return "", nil
	}
	local, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok {
		return "", nil
	}
	return local.Name, caseLiteral(assignment.Rhs[0])
}

func startsWithBreak(statement ast.Stmt) bool {
	loop, ok := statement.(*ast.ForStmt)
	if !ok || loop.Init != nil || loop.Cond != nil || len(loop.Body.List) == 0 {
		return false
	}
	guard, ok := loop.Body.List[0].(*ast.IfStmt)
	if !ok || guard.Init != nil || len(guard.Body.List) != 1 {
		return false
	}
	exit, ok := guard.Body.List[0].(*ast.BranchStmt)
	return ok && exit.Tok == token.BREAK && exit.Label == nil
}

func caseLiteral(expression ast.Expr) *ast.CompositeLit {
	literal, _ := ast.Unparen(expression).(*ast.CompositeLit)
	return literal
}
