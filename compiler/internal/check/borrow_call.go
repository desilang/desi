package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// baseLvalue returns the base storage name if e is an lvalue we track.
// Accepted lvalues for M6-P2-B: Ident, FieldExpr(base, .name), IndexExpr(base, [idx]).
// For Field/Index we return the base's name (recursive).
func (c *checker) baseLvalue(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.FieldExpr:
		return c.baseLvalueExpr(v.X)
	case *ast.IndexExpr:
		return c.baseLvalueExpr(v.X)
	default:
		return "", false
	}
}

// helper to recurse; separated to share with both Field and Index branches.
func (c *checker) baseLvalueExpr(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.FieldExpr:
		return c.baseLvalueExpr(v.X)
	case *ast.IndexExpr:
		return c.baseLvalueExpr(v.X)
	default:
		return "", false
	}
}

// enforceCallModes applies caller-side borrow/move rules given the chosen overload.
func (c *checker) enforceCallModes(call *ast.CallExpr, cand *FuncCand) {
	if call == nil || cand == nil {
		return
	}
	// Defensive: ensure we have a modes slice aligned with args.
	modes := cand.Modes
	if len(modes) == 0 {
		// Default to move semantics if modes are absent.
		modes = make([]ast.ParamMode, len(call.Args))
		for i := range modes {
			modes[i] = ast.ParamMove
		}
	}
	// Aliasing tracker: base name -> mode encountered
	seen := make(map[string]ast.ParamMode)

	for i := 0; i < len(call.Args) && i < len(modes); i++ {
		arg := call.Args[i]
		mode := modes[i]

		// (1) inout requires lvalue
		if mode == ast.ParamInout {
			if base, ok := c.baseLvalue(arg); !ok {
				c.add(diagAt("DBR0002", arg.SpanOf(), "inout argument must be a mutable lvalue"))
			} else {
				// (2) aliasing: inout cannot alias with any prior arg of same base; also reject inout+ref aliasing
				if prev, exists := seen[base]; exists {
					if prev == ast.ParamInout || prev == ast.ParamRef || mode == ast.ParamRef || mode == ast.ParamInout {
						c.add(diagAt("DBR0003", arg.SpanOf(), "inout cannot alias with "+base+" in the same call"))
					}
				}
				seen[base] = mode
			}
		} else {
			// Track base for aliasing with future inout.
			if base, ok := c.baseLvalue(arg); ok {
				if prev, exists := seen[base]; !exists {
					seen[base] = mode
				} else {
					// If we already saw this base and either side is inout/ref+inout, flag when the current one is inout.
					if mode == ast.ParamInout || prev == ast.ParamInout || mode == ast.ParamRef {
						c.add(diagAt("DBR0003", arg.SpanOf(), "inout cannot alias with "+base+" in the same call"))
					}
				}
			}
		}

		// (3) move tracking: identifier passed to a move param is considered moved.
		if mode == ast.ParamMove {
			if id, ok := arg.(*ast.Ident); ok {
				if c.info != nil {
					if c.info.Moved == nil {
						c.info.Moved = make(map[string]diag.Span)
					}
					c.info.Moved[id.Name] = id.Span
				}
			}
		}
	}
}
