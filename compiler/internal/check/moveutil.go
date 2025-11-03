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

// isCopyType returns true for value types that do NOT "move" on pass-by-value.
// M6 polish: primitives are copy types: int, float, bool, str.
// Everything else remains move-by-default for now.
func isCopyType(t types.T) bool {
	if t == nil {
		return false
	}
	return types.Equal(t, types.Int) ||
		types.Equal(t, types.Float) ||
		types.Equal(t, types.Bool) ||
		types.Equal(t, types.Str)
}

// markMovesFromCall marks identifiers passed to "move" params of a local function.
//
// We only mark moves when:
//   - The chosen candidate is a local decl (cand.Decl != nil), so we know true param modes.
//   - The argument is an lvalue name (we don't bother tracking temporaries).
//   - The argument type is NOT a copy type (copy types are ignored).
func (c *checker) markMovesFromCall(chosen *FuncCand, call *ast.CallExpr, args []types.T) {
	// Only for in-module functions (we can read modes from the Decl).
	if chosen == nil || chosen.Decl == nil || call == nil {
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
			continue // borrows do not move
		}
		// Default mode == "move".
		arg := call.Args[i]
		name, ok := c.baseLvalue(arg)
		if !ok {
			// Moving a temporary is allowed and we don't track it.
			continue
		}
		// Skip marking for copy types (int/float/bool/str).
		if i < len(args) && isCopyType(args[i]) {
			continue
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
