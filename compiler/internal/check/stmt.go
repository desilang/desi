package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// NOTE: unchanged cases elided for brevity in your view; this is a full function.
// Paste the whole thing, replacing your existing checkStmt entirely.
func (c *checker) checkStmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.LetStmt:
		var rhs types.T
		if st.Value != nil {
			rhs = c.typ(st.Value)
		}
		var t types.T
		if st.Type != nil {
			t, _ = types.FromName(st.Type.Name)
			// Use new helper for parameterized types
			if t == nil {
				t = fromTypeName(st.Type)
			}
			if rhs != nil && !types.Assignable(t, rhs) {
				c.add(diagAt("DTE0004", st.Span, "cannot assign '"+rhs.String()+"' to '"+t.String()+"'"))
			}
		} else {
			t = rhs
		}
		sym := &Symbol{Name: st.Name.Name, Kind: SymVar, Type: t, Node: st}
		_ = c.scope.Define(sym)

		// Enrich Info: attach symbol/type to the declared name node
		c.info.Idents[&st.Name] = sym
		if t != nil {
			c.info.Types[&st.Name] = t
		}

	case *ast.AssignStmt:
		// width must match
		if len(st.LHS) != len(st.RHS) {
			c.add(diagAt("DTE0002", st.Span, "arity mismatch in assignment"))
			return
		}
		for i := range st.LHS {
			lt := st.LHS[i]
			rt := st.RHS[i]
			// Only support identifier targets in M4
			id, ok := lt.(*ast.Ident)
			if !ok {
				_ = c.typ(lt)
				_ = c.typ(rt)
				continue
			}
			valT := c.typ(rt)
			sym := c.scope.Lookup(id.Name)
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined name: "+id.Name))
				continue
			}
			c.info.Idents[id] = sym
			if sym.Type != nil {
				c.info.Types[id] = sym.Type
			}
			if !types.Assignable(sym.Type, valT) {
				c.add(diagAt("DTE0004", st.Span, "cannot assign '"+valT.String()+"' to '"+sym.Type.String()+"'"))
			}
		}

	case *ast.AugAssignStmt:
		lt := c.typ(st.Left)
		rt := c.typ(st.Right)
		op := st.Op
		// Treat as binary op type check on the underlying op (e.g., "+=" -> "+").
		be := &ast.BinaryExpr{Op: op[:len(op)-1], Lhs: st.Left, Rhs: st.Right, Span: st.Span}
		_ = c.typBinary(be)

		if lt != nil && rt != nil {
			// We expect the binary result type to be the same as LHS for a valid AugAssign.
			if resT, ok := beResultType(be.Op, lt, rt); !ok || !types.Equal(lt, resT) {
				c.add(diagAt("DTE0004", st.Span, "invalid augmented assignment"))
			}
		}

	case *ast.ReturnStmt:
		if st.Value == nil {
			// returning none is always fine if declared none
			if c.curFuncRet != nil && !types.Equal(c.curFuncRet, types.None) {
				c.add(diagAt("DTE0005", st.Span, "missing return value"))
			}
			return
		}
		rt := c.typ(st.Value)
		if c.curFuncRet != nil && !types.Equal(rt, c.curFuncRet) {
			c.add(diagAt("DTE0004", st.Span, "wrong return type: expected "+c.curFuncRet.String()))
		}

	case *ast.ExprStmt:
		_ = c.typ(st.Expr)

	case *ast.IfStmt:
		_ = c.typ(st.Cond)
		if st.Then != nil {
			c.checkBlock(st.Then)
		}
		for _, arm := range st.Elifs {
			_ = c.typ(arm.Cond)
			if arm.Body != nil {
				c.checkBlock(arm.Body)
			}
		}
		if st.Else != nil {
			c.checkBlock(st.Else)
		}

	case *ast.MatchStmt:
		// M4: we only require all arm result types to match; do NOT type the scrutinee here.
		var want types.T
		for _, arm := range st.Arms {
			if arm.Result == nil {
				continue // allow empty/side-effect arms for now
			}
			at := c.typ(arm.Result)
			if want == nil {
				want = at
				continue
			}
			if at != nil && !types.Equal(at, want) {
				c.add(diagAt("DTE0004", arm.Result.SpanOf(),
					"match arm type mismatch"))
			}
		}

	case *ast.WhileStmt:
		_ = c.typ(st.Cond)
		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.ForStmt:
		_ = c.typ(st.Iter)
		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.DeferStmt:
		if st.Call != nil {
			_ = c.typ(st.Call)
		}

	case *ast.UsingStmt:
		if st.Bind != nil {
			_ = c.typ(st.Bind)
		}
		if st.Init != nil {
			_ = c.typ(st.Init)
		}
		if st.Body != nil {
			c.checkBlock(st.Body)
		}

	case *ast.UnsafeBlock:
		// Enter unsafe context for the duration of the block.
		c.unsafeDepth++
		if st.Body != nil {
			c.checkBlock(st.Body)
		}
		c.unsafeDepth--

	// NEW (M5): imports
	case *ast.ImportStmt:
		// nothing to type; resolver handles validity; lints post-check
	case *ast.FromImportStmt:
		// nothing to type; resolver handles validity; lints post-check
	}
}

// beResultType returns (resultType, ok) for a binary operator (op) applied to (lt, rt).
// Phase-1: exact same-type numeric ops; string+string for "+"; boolean on logical ops & equality for same primitives.
func beResultType(op string, lt, rt types.T) (types.T, bool) {
	switch op {
	case "+": // addition OR string concatenation
		// numeric + numeric (same type)
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		if types.Equal(lt, types.Float) && types.Equal(rt, types.Float) {
			return types.Float, true
		}
		// string + string -> string
		if types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			return types.Str, true
		}
		return nil, false

	case "-", "*", "/", "%":
		// numeric only; same-type
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		if types.Equal(lt, types.Float) && types.Equal(rt, types.Float) {
			return types.Float, true
		}
		return nil, false

	case "|", "&", "^":
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		return nil, false

	case "<<", ">>":
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int, true
		}
		return nil, false

	case "==", "!=":
		// Equality only on same primitives (int/float/bool/str) for this phase.
		if types.Equal(lt, rt) && (isPrimitive(lt)) {
			return types.Bool, true
		}
		return nil, false

	case "<", "<=", ">", ">=":
		// Numeric comparisons only; same-type
		if (types.Equal(lt, types.Int) && types.Equal(rt, types.Int)) ||
			(types.Equal(lt, types.Float) && types.Equal(rt, types.Float)) {
			return types.Bool, true
		}
		return nil, false

	case "and", "or":
		if types.Equal(lt, types.Bool) && types.Equal(rt, types.Bool) {
			return types.Bool, true
		}
		return nil, false

	case "|>":
		// Pipeline typed elsewhere
		return nil, false

	default:
		return nil, false
	}
}

// helper: primitive means the simple builtins we handle here.
func isPrimitive(t types.T) bool {
	return types.Equal(t, types.Int) ||
		types.Equal(t, types.Float) ||
		types.Equal(t, types.Bool) ||
		types.Equal(t, types.Str)
}
