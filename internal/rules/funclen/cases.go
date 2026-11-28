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
	var previous ast.Stmt
	for _, stmt := range statements {
		if loop, ok := stmt.(*ast.RangeStmt); ok {
			if table := adjacentCaseTable(previous, loop.X); table != nil {
				found[table] = struct{}{}
			}
		}
		previous = stmt
	}
}

func adjacentCaseTable(previous ast.Stmt, expression ast.Expr) *ast.CompositeLit {
	name, ok := ast.Unparen(expression).(*ast.Ident)
	if !ok {
		return nil
	}
	declaration, ok := previous.(*ast.AssignStmt)
	if !ok || declaration.Tok != token.DEFINE {
		return nil
	}
	if len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 {
		return nil
	}
	local, ok := declaration.Lhs[0].(*ast.Ident)
	if !ok || local.Name != name.Name {
		return nil
	}
	return caseLiteral(declaration.Rhs[0])
}

func caseLiteral(expression ast.Expr) *ast.CompositeLit {
	literal, _ := ast.Unparen(expression).(*ast.CompositeLit)
	return literal
}
