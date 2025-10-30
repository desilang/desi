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
func (c *checker) typBinary(x *ast.BinaryExpr) types.T {
	op := x.Op

	switch op {
	case "+", "-", "*", "/", "%", "**":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// string concatenation
		if op == "+" && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}

		// numeric same-type
		if (types.Equal(lt, types.Int) || types.Equal(lt, types.Float)) && types.Equal(lt, rt) {
			c.info.Types[x] = lt
			return lt
		}
		c.add(diagAt("DTE0004", x.Span, "invalid operands for '"+op+"'"))
		return nil

	case "==", "!=":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		// permit equality on same primitive types
		if types.Equal(lt, rt) && (types.Equal(lt, types.Int) || types.Equal(lt, types.Float) || types.Equal(lt, types.Bool) || types.Equal(lt, types.Str)) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "invalid comparison"))
		return nil

	case "<", "<=", ">", ">=":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if (types.Equal(lt, types.Int) && types.Equal(rt, types.Int)) ||
			(types.Equal(lt, types.Float) && types.Equal(rt, types.Float)) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "invalid comparison"))
		return nil

	case "|", "&", "^":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			c.info.Types[x] = types.Int
			return types.Int
		}
		c.add(diagAt("DTE0004", x.Span, "bitwise operators require int operands"))
		return nil

	case "<<", ">>":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if types.Equal(lt, types.Int) && types.Equal(rt, types.Int) {
			c.info.Types[x] = types.Int
			return types.Int
		}
		c.add(diagAt("DTE0004", x.Span, "bitwise operators require int operands"))
		return nil

	case "|>":
		// pipeline: lhs |> f(a,b)  ==>  f(lhs, a, b)
		// Ensure RHS is a call
		call, ok := x.Rhs.(*ast.CallExpr)
		if !ok {
			c.add(diagAt("DTE0103", x.Span, "pipeline expects a call on the right-hand side"))
			return nil
		}
		// Callee must be identifier in Phase-1
		id, ok := call.Callee.(*ast.Ident)
		if !ok {
			c.add(diagAt("DTE0103", x.Span, "pipeline target must be an identifier"))
			return nil
		}
		// Build synthetic call with lhs as first arg
		args := make([]ast.Expr, 0, 1+len(call.Args))
		args = append(args, x.Lhs)
		args = append(args, call.Args...)
		synth := &ast.CallExpr{Callee: id, Args: args, Span: call.Span}

		ret := c.typCall(synth)
		if ret == nil {
			// Bubble a pipeline-flavored message so tests match on "pipeline".
			c.add(diagAt("DTE0046", x.Span, "pipeline: type/arity error"))
			return nil
		}
		c.info.Types[x] = ret
		return ret
	}

	// unknown binary op (not handled)
	return nil
}

func (c *checker) typCall(call *ast.CallExpr) types.T {

	// Module-qualified callee: e.g., math.add(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			// 1) Gather arg types.
			args := make([]types.T, len(call.Args))
			for i, a := range call.Args {
				args[i] = c.typ(a)
			}
			// 2) If no exported candidates, it's a bad import member.
			if set == nil || len(set.Cands) == 0 {
				c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
				return nil
			}
			// 3) Exact-match overload resolution.
			cands := set.ResolveExact(args)
			switch len(cands) {
			case 1:
				ret := cands[0].Type.Ret
				c.info.Types[call] = ret
				return ret
			case 0:
				// Distinguish arity vs type mismatch for clearer errors.
				sameArity := false
				for _, cand := range set.Cands {
					if len(cand.Type.Params) == len(args) {
						sameArity = true
						break
					}
				}
				if !sameArity {
					c.add(diagAt("DTE0046", call.Span, "arity mismatch for call to "+fe.Name.Name))
				} else {
					c.add(diagAt("DTE0101", call.Span, "no matching overload for call to "+fe.Name.Name))
				}
				return nil
			default:
				c.add(diagAt("DTE0102", call.Span, "ambiguous overload for call to "+fe.Name.Name))
				return nil
			}
		}
	}

	// Identifier callee path
	if id, ok := call.Callee.(*ast.Ident); ok {
		set, hasSet := c.info.Funcs[id.Name]

		// Resolve scope symbol if any.
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc

		// Determine callability:
		// - A true function symbol is callable.
		// - Builtins may not have a symbol bound in scope; allow calling if we have overloads in Info.Funcs.
		// - From-import aliases are allowed if there is an Info.Funcs entry (even empty), to enable Phase-1 fallback.
		callable := isCallableSym || (hasSet && (set != nil))

		if !callable {
			// Prefer "undefined function" if there's no symbol and no prelude/alias record.
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			// Symbol exists but is not a function and no callable record -> not callable.
			c.add(diagAt("DTE0045", id.Span, "object is not callable"))
			return nil
		}

		// 2) Gather arg types.
		args := make([]types.T, len(call.Args))
		for i, a := range call.Args {
			args[i] = c.typ(a)
		}

		// 3) Overload resolution (or Phase-1 permissive fallback for alias/bare names).
		// If we have no candidate signatures, use permissive same-primitive passthrough.
		if set == nil || len(set.Cands) == 0 {
			if len(args) == 0 {
				c.info.Types[call] = types.None
				return types.None
			}
			base := args[0]
			isPrim := types.Equal(base, types.Int) || types.Equal(base, types.Float) ||
				types.Equal(base, types.Bool) || types.Equal(base, types.Str)
			if isPrim {
				allSame := true
				for i := 1; i < len(args); i++ {
					if !types.Equal(args[i], base) {
						allSame = false
						break
					}
				}
				if allSame {
					c.info.Types[call] = base
					return base
				}
			}
			// Can't infer safely; leave unknown (no extra diag here).
			return nil
		}

		// Resolve exact-match overload.
		cands := set.ResolveExact(args)
		switch len(cands) {
		case 1:
			ret := cands[0].Type.Ret
			c.info.Types[call] = ret
			return ret
		case 0:
			// Distinguish arity vs type mismatch for clearer errors.
			sameArity := false
			for _, cand := range set.Cands {
				if len(cand.Type.Params) == len(args) {
					sameArity = true
					break
				}
			}
			if !sameArity {
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

	// Non-identifier callee: first-class function value?
	ct := c.typ(call.Callee)
	if fn, ok := ct.(*types.Func); ok {
		args := make([]types.T, len(call.Args))
		for i, a := range call.Args {
			args[i] = c.typ(a)
		}
		if len(fn.Params) != len(args) {
			c.add(diagAt("DTE0046", call.Span, "arity mismatch"))
			return nil
		}
		for i := range args {
			if !types.Equal(args[i], fn.Params[i]) {
				c.add(diagAt("DTE0101", call.Span, "no matching overload"))
				return nil
			}
		}
		c.info.Types[call] = fn.Ret
		return fn.Ret
	}

	// Not callable at all.
	c.add(diagAt("DTE0045", call.Span, "object is not callable"))
	return nil
}
