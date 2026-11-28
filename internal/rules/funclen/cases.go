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
	for i, stmt := range statements {
		if loop, ok := unlabel(stmt).(*ast.RangeStmt); ok {
			if table := adjacentCaseTable(statements[:i], loop.X); table != nil {
				found[table] = struct{}{}
			}
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

func adjacentCaseTable(before []ast.Stmt, expression ast.Expr) *ast.CompositeLit {
	name := rangedName(expression)
	count := len(before)
	if name == "" || count == 0 {
		return nil
	}
	previous := before[count-1]
	if value := declaredValue(previous, name); value != nil {
		if table := caseLiteral(value); table != nil {
			return table
		}
		aliasOf, ok := ast.Unparen(value).(*ast.Ident)
		if ok && count >= 2 {
			return caseLiteral(declaredValue(before[count-2], aliasOf.Name))
		}
	}
	if count >= 2 && startsWithBreak(previous) {
		return caseLiteral(declaredValue(before[count-2], name))
	}
	return nil
}

func rangedName(expression ast.Expr) string {
	expression = ast.Unparen(expression)
	if slice, ok := expression.(*ast.SliceExpr); ok {
		if slice.Low != nil || slice.High != nil {
			return ""
		}
		expression = ast.Unparen(slice.X)
	}
	if name, ok := expression.(*ast.Ident); ok {
		return name.Name
	}
	return ""
}

func declaredValue(statement ast.Stmt, name string) ast.Expr {
	declaration, ok := statement.(*ast.AssignStmt)
	if !ok || declaration.Tok != token.DEFINE {
		return nil
	}
	if len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 {
		return nil
	}
	local, ok := declaration.Lhs[0].(*ast.Ident)
	if !ok || local.Name != name {
		return nil
	}
	return declaration.Rhs[0]
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
