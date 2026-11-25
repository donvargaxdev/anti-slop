package tests

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (c flakeChecks) isNil(expr ast.Expr) bool {
	name, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && c.info.Uses[name] == types.Universe.Lookup("nil")
}

func (c flakeChecks) knownNilError(expr ast.Expr) bool {
	name, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && c.outcomes[c.info.ObjectOf(name)] == nilError
}

func (c *flakeChecks) constrainError(expr ast.Expr, equalToNil bool) {
	name, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok || c.outcomes == nil || !c.tracksNilError(c.info.ObjectOf(name)) {
		return
	}
	if equalToNil {
		c.storeOutcome(name, nilError)
	} else if c.knownNilError(name) {
		c.outcomes = nil
	}
}

func (c flakeChecks) tracksNilError(object types.Object) bool {
	variable, ok := object.(*types.Var)
	return ok && isLocalError(variable) && !c.sharedErrors[variable]
}

func isLocalError(variable *types.Var) bool {
	// Boxing a nil concrete value in an error interface produces a non-nil error.
	iface, ok := variable.Type().Underlying().(*types.Interface)
	return ok && iface.IsMethodSet() && isErrorResult(variable.Type()) &&
		variable.Pkg() != nil && variable.Parent() != variable.Pkg().Scope()
}

// Addressed and captured errors can change outside the assignments this walk sees.
func sharedErrorVariables(info *types.Info, file *ast.File) map[types.Object]bool {
	shared := make(map[types.Object]bool)
	ast.PreorderStack(file, nil, func(node ast.Node, parents []ast.Node) bool {
		switch expr := node.(type) {
		case *ast.UnaryExpr:
			if name, ok := ast.Unparen(expr.X).(*ast.Ident); ok && expr.Op == token.AND {
				if variable, ok := info.Uses[name].(*types.Var); ok && isLocalError(variable) {
					shared[variable] = true
				}
			}
		case *ast.Ident:
			if variable, ok := info.Uses[expr].(*types.Var); ok && isLocalError(variable) {
				for _, parent := range parents {
					if literal, ok := parent.(*ast.FuncLit); ok && variable.Pos() < literal.Pos() {
						shared[variable] = true
						break
					}
				}
			}
		}
		return true
	})
	return shared
}
