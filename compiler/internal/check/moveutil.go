package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

// MoveSet tracks identifiers that were moved, keyed by base name.
type MoveSet struct {
	m map[string]diag.Span
}

func (ms *MoveSet) mark(name string, at diag.Span) {
	if ms.m == nil {
		ms.m = make(map[string]diag.Span)
	}
	ms.m[name] = at
}

func (ms *MoveSet) movedAt(name string) (diag.Span, bool) {
	if ms == nil || ms.m == nil {
		return diag.Span{}, false
	}
	sp, ok := ms.m[name]
	return sp, ok
}

// isCopyType: conservative for now (everything moves). We'll flip primitives to Copy later.
func isCopyType(t types.T) bool {
	return false
}

// markMovesFromCall marks identifiers passed to "move" params of a local function.
func (c *checker) markMovesFromCall(chosen *FuncCand, call *ast.CallExpr, args []types.T) {
	// Only for in-module functions (we can read modes from the Decl).
	if chosen == nil || chosen.Decl == nil {
		return
	}
	fd := chosen.Decl
	n := len(fd.Params)
	if n > len(call.Args) {
		n = len(call.Args)
	}
	for i := 0; i < n; i++ {
		mode := fd.Params[i].Mode
		if mode == ast.ParamRef || mode == ast.ParamInout {
			continue // not a move
		}
		// (mode == default move)
		arg := call.Args[i]
		name, ok := c.baseLvalue(arg) // uses the existing baseLvalue in borrow_call.go
		if !ok {
			continue // moving a temporary is fine; only track named bases
		}
		if i < len(args) && isCopyType(args[i]) {
			continue // when we flip primitives to Copy, this will skip them
		}
		c.moved.mark(name, arg.SpanOf())
	}
}

// issueUseAfterMove emits DBR0004 with a secondary "moved here" label.
func (c *checker) issueUseAfterMove(useSpan diag.Span, moveSpan diag.Span) {
	d := diagAt("DBR0004", useSpan, "value was moved earlier and cannot be used again")
	d.Labels = append(d.Labels, diag.Label{
		Span:    moveSpan,
		Text:    "moved here",
		Primary: false,
	})
	c.add(d)
}
