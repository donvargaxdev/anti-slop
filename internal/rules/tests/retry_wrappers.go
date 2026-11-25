package tests

import (
	"go/ast"
	"go/types"
	"strings"
)

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
