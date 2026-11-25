package tests

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"
)

func TestDeadlineScanWithUnmatchedBreakLabel(t *testing.T) {
	label := ast.NewIdent("outer")
	target := types.NewLabel(token.NoPos, nil, "outer")
	channel := types.NewVar(token.NoPos, nil, "deadline", types.NewChan(types.RecvOnly, types.Typ[types.Int]))
	scan := pollingScan{
		info:    &types.Info{Uses: map[*ast.Ident]types.Object{label: target}},
		tracked: map[types.Object]bool{channel: true},
	}

	// Type-checked fixtures cannot contain a break to an unregistered label.
	loop := &ast.ForStmt{Body: &ast.BlockStmt{List: []ast.Stmt{
		&ast.BranchStmt{Tok: token.BREAK, Label: label},
	}}}
	scan.record(loop)
	if !scan.tracked[channel] {
		t.Error("the unresolved branch lost the incoming deadline")
	}
}
