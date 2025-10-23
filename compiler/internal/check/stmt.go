package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

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
		_ = c.scope.Define(&Symbol{Name: st.Name.Name, Kind: SymVar, Type: t, Node: st})
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
			if !types.Assignable(sym.Type, valT) {
				c.add(diagAt("DTE0004", st.Span, "cannot assign '"+valT.String()+"' to '"+sym.Type.String()+"'"))
			}
		}
	case *ast.AugAssignStmt:
		lt := c.typ(st.Left)
		rt := c.typ(st.Right)
		op := st.Op
		// treat as binary op type check
		be := &ast.BinaryExpr{Op: op[:len(op)-1], Lhs: st.Left, Rhs: st.Right, Span: st.Span}
		_ = c.typBinary(be)
		// result must be assignable back to left type; under our rules, it's same type when valid
		if lt != nil && rt != nil && !types.Equal(lt, beResultType(lt, rt, be.Op)) {
			// conservative mismatch message
			c.add(diagAt("DTE0004", st.Span, "invalid augmented assignment"))
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
	case *ast.UsingStmt:
		// Type init and body for now
		_ = c.typ(st.Init)
		c.checkBlock(st.Body)
	case *ast.DeferStmt:
		if st.Call != nil {
			_ = c.typCall(st.Call)
		}
	default:
		// unhandled statements are ignored in M4
		_ = st
	}
}

// beResultType returns the expected result type of a binary op under our simple rules.
func beResultType(lt, rt types.T, op string) types.T {
	switch op {
	case "+", "-", "*", "/", "%", "**":
		if types.Equal(lt, rt) && (types.Equal(lt, types.Int) || types.Equal(lt, types.Float)) {
			return lt
		}
	case "^", "|":
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			return types.Int
		}
	}
	return nil
}
