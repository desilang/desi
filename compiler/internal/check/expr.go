package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) typ(e ast.Expr) types.T {
	switch x := e.(type) {
	case *ast.IntLit:
		c.info.Types[e] = types.Int
		return types.Int
	case *ast.FloatLit:
		c.info.Types[e] = types.Float
		return types.Float
	case *ast.BoolLit:
		c.info.Types[e] = types.Bool
		return types.Bool
	case *ast.StrLit:
		c.info.Types[e] = types.Str
		return types.Str
	case *ast.NoneLit:
		c.info.Types[e] = types.None
		return types.None
	case *ast.Ident:
		// Lookup in scope
		sym := c.scope.Lookup(x.Name)
		if sym == nil {
			c.add(diagAt("DTE0001", x.Span, "undefined name: "+x.Name))
			return nil
		}
		c.info.Idents[x] = sym
		c.info.Types[e] = sym.Type
		return sym.Type
	case *ast.UnaryExpr:
		// Only support '-' on numeric and '!'/'not' on bool
		t := c.typ(x.X)
		if x.Op == "-" {
			if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
				c.info.Types[e] = t
				return t
			}
		} else if x.Op == "!" || x.Op == "not" {
			if types.Equal(t, types.Bool) {
				c.info.Types[e] = types.Bool
				return types.Bool
			}
		}
		c.add(diagAt("DTE0004", x.Span, "invalid operand for unary operator '"+x.Op+"'"))
		return nil
	case *ast.BinaryExpr:
		return c.typBinary(x)
	case *ast.CallExpr:
		return c.typCall(x)
	case *ast.FieldExpr:
		// Not modeled in M4; leave unknown
		return nil
	case *ast.IndexExpr:
		// Not modeled in M4; leave unknown
		return nil
	case *ast.LambdaExpr:
		// require typed params in M4
		params := make([]types.T, len(x.Params))
		for i, p := range x.Params {
			if p.Type == nil {
				c.add(diagAt("DTE0004", x.Span, "lambda parameters must be typed in M4"))
				return nil
			}
			pt, _ := types.FromName(p.Type.Name)
			params[i] = pt
		}
		ret := c.typ(x.Body)
		ft := types.FuncOf(params, ret)
		c.info.Types[e] = ft
		return ft
	case *ast.ListComp:
		et := c.typ(x.Elem)
		lt := types.ListOf(et)
		c.info.Types[e] = lt
		return lt
	case *ast.SetComp:
		et := c.typ(x.Elem)
		st := types.SetOf(et)
		c.info.Types[e] = st
		return st
	case *ast.DictComp:
		kt := c.typ(x.Key)
		vt := c.typ(x.Val)
		dt := types.DictOf(kt, vt)
		c.info.Types[e] = dt
		return dt
	default:
		return nil
	}
}

// compiler/internal/check/expr.go — replace the entire typBinary function with this.
func (c *checker) typBinary(be *ast.BinaryExpr) types.T {
	lt := c.typ(be.Lhs)
	rt := c.typ(be.Rhs)

	// Pipeline sugar:  a |> f(x, y)  ≡  f(a, x, y)
	if be.Op == "|>" {
		// RHS must be a call.
		call, ok := be.Rhs.(*ast.CallExpr)
		if !ok {
			c.add(diagAt("DTE0103", be.Span, "pipeline expects a call on the right-hand side"))
			return nil
		}
		// For M4/M5, require simple identifier as callee (no dotted/field calls yet).
		id, ok := call.Callee.(*ast.Ident)
		if !ok {
			c.add(diagAt("DTE0103", be.Span, "pipeline target must be an identifier"))
			return nil
		}
		// Build the effective argument list: [piped] + existing args
		args := make([]types.T, 1+len(call.Args))
		args[0] = c.typ(be.Lhs)
		for i, a := range call.Args {
			args[i+1] = c.typ(a)
		}

		set := c.info.Funcs[id.Name]
		if set == nil {
			c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
			return nil
		}
		cands := set.ResolveExact(args)
		switch len(cands) {
		case 1:
			ret := cands[0].Type.Ret
			c.info.Types[be] = ret
			return ret
		case 0:
			// Distinguish arity vs. type mismatch for nicer UX/tests.
			sameArity := false
			for _, cand := range set.Cands {
				if len(cand.Type.Params) == len(args) {
					sameArity = true
					break
				}
			}
			if !sameArity {
				c.add(diagAt("DTE0046", be.Span, "arity mismatch for pipeline"))
			} else {
				c.add(diagAt("DTE0101", be.Span, "no matching overload for pipeline"))
			}
			return nil
		default:
			c.add(diagAt("DTE0102", be.Span, "ambiguous overload for pipeline"))
			return nil
		}
	}

	// Non-pipeline operators: delegate to the simple table and surface invalid operand types.
	if t, ok := beResultType(be.Op, lt, rt); ok {
		c.info.Types[be] = t
		return t
	}
	c.add(diagAt("DTE0104", be.Span, "invalid operand types for '"+be.Op+"'"))
	return nil
}

func (c *checker) typCall(call *ast.CallExpr) types.T {
	// Only handle ident callees for M4
	id, ok := call.Callee.(*ast.Ident)
	if !ok {
		// Try callee as first-class function
		ct := c.typ(call.Callee)
		if fn, ok := ct.(*types.Func); ok {
			args := make([]types.T, len(call.Args))
			for i, a := range call.Args {
				args[i] = c.typ(a)
			}
			if len(args) != len(fn.Params) {
				c.add(diagAt("DTE0046", call.Span, "arity mismatch"))
				return nil
			}
			for i := range args {
				if !types.Equal(args[i], fn.Params[i]) {
					c.add(diagAt("DTE0004", call.Span, "argument type mismatch"))
					return nil
				}
			}
			c.info.Types[call] = fn.Ret
			return fn.Ret
		}
		c.add(diagAt("DTE0105", call.Span, "expression is not callable"))
		return nil
	}

	set := c.info.Funcs[id.Name]
	if set == nil {
		c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
		return nil
	}

	args := make([]types.T, len(call.Args))
	for i, a := range call.Args {
		args[i] = c.typ(a)
	}

	// Try exact match first.
	cands := set.ResolveExact(args)
	switch len(cands) {
	case 1:
		c.info.Types[call] = cands[0].Type.Ret
		return cands[0].Type.Ret
	case 0:
		// No exact match—distinguish arity vs. type mismatch.
		hasSameArity := false
		for _, cand := range set.Cands {
			if len(cand.Type.Params) == len(args) {
				hasSameArity = true
				break
			}
		}
		if !hasSameArity {
			c.add(diagAt("DTE0046", call.Span, "arity mismatch for call to "+id.Name))
		} else {
			c.add(diagAt("DTE0101", call.Span, "no matching overload for call to "+id.Name))
		}
		return nil
	default:
		c.add(diagAt("DTE0102", call.Span, "ambiguous overload for call to "+id.Name))
		return nil
	}
}
