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
				c.add(diagAt("DTE0031", st.Span, "unsupported assignment target"))
				continue
			}
			valT := c.typ(rt)
			sym := c.scope.Lookup(id.Name)
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined name: "+id.Name))
				continue
			}
			// Enrich Info on LHS ident as well
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
		vt := c.typ(st.Value)
		if c.curFuncRet != nil && vt != nil && !types.Assignable(c.curFuncRet, vt) {
			c.add(diagAt("DTE0005", st.Span, "return type mismatch: expected '"+c.curFuncRet.String()+"', found '"+vt.String()+"'"))
		}

	case *ast.ExprStmt:
		_ = c.typ(st.Expr)

	case *ast.IfStmt:
		ct := c.typ(st.Cond)
		if !types.Equal(ct, types.Bool) {
			c.add(diagAt("DTE0004", st.Cond.SpanOf(), "if condition must be bool"))
		}
		c.checkBlock(st.Then)
		for _, arm := range st.Elifs {
			ct := c.typ(arm.Cond)
			if !types.Equal(ct, types.Bool) {
				c.add(diagAt("DTE0004", arm.Cond.SpanOf(), "elif condition must be bool"))
			}
			c.checkBlock(arm.Body)
		}
		c.checkBlock(st.Else)

	case *ast.WhileStmt:
		ct := c.typ(st.Cond)
		if !types.Equal(ct, types.Bool) {
			c.add(diagAt("DTE0004", st.Cond.SpanOf(), "while condition must be bool"))
		}
		c.checkBlock(st.Body)

	case *ast.ForStmt:
		// Type the iterable and body; skip target checks in M4
		_ = c.typ(st.Iter)
		c.checkBlock(st.Body)

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
					"match arm result type mismatch: expected '"+want.String()+"', found '"+at.String()+"'"))
			}
		}

	default:
		// no-op for other statements in M4
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
