package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) typ(e ast.Expr) types.T {
	switch x := e.(type) {
	case *ast.SliceExpr:
		// M5 P4d: basic typing for slice steps.
		bt := c.typ(x.X)
		if x.I != nil {
			_ = c.typ(x.I)
		}
		if x.J != nil {
			_ = c.typ(x.J)
		}
		if x.K != nil {
			_ = c.typ(x.K)
		}
		if types.Equal(bt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}
		return nil

	case *ast.ListComp:
		et := c.typ(x.Elem)
		if et != nil {
			t := types.ListOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.SetComp:
		et := c.typ(x.Elem)
		if et != nil {
			t := types.SetOf(et)
			c.info.Types[x] = t
			return t
		}
		return nil

	case *ast.DictComp:
		kt := c.typ(x.Key)
		vt := c.typ(x.Val)
		if kt != nil && vt != nil {
			t := types.DictOf(kt, vt)
			c.info.Types[x] = t
			return t
		}
		return nil

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
		return c.typIdent(x)

	case *ast.UnaryExpr:
		t := c.typ(x.X)
		if x.Op == "-" {
			if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
				c.info.Types[e] = t
				return t
			}
		}
		if x.Op == "!" || x.Op == "not" {
			if types.Equal(t, types.Bool) {
				c.info.Types[e] = types.Bool
				return types.Bool
			}
		}
		c.add(diagAt("DTE0004", x.Span, "invalid unary '"+x.Op+"'"))
		return nil

	case *ast.BinaryExpr:
		return c.typBinary(x)

	case *ast.CallExpr:
		return c.typCall(x)

	case *ast.FieldExpr:
		// Not modeled yet
		return nil
	case *ast.IndexExpr:
		// Not modeled yet
		return nil

	case *ast.LambdaExpr:
		// require typed params in M4
		params := make([]types.T, len(x.Params))
		for i, p := range x.Params {
			if p.Type == nil {
				c.add(diagAt("DTE0004", x.Span, "lambda parameters must be typed in M4"))
				return nil
			}
			if t, ok := types.FromName(p.Type.Name); ok {
				params[i] = t
			} else {
				c.add(diagAt("DTE0004", x.Span, "unknown lambda param type: "+p.Type.Name))
				return nil
			}
		}
		bt := c.typ(x.Body)
		c.info.Types[e] = types.FuncOf(params, bt)
		return c.info.Types[e]

	default:
		// Unknown or unmodeled node
		return nil
	}
}

// typIdent handles identifier expressions, including DBR0004 (use after move).
func (c *checker) typIdent(x *ast.Ident) types.T {
	// If this name was previously moved, emit DBR0004 with a note.
	if sp, ok := c.moved.movedAt(x.Name); ok {
		c.issueUseAfterMove(x.Span, sp)
	}

	// If it's bound in the current scope, use the symbol's type.
	if sym := c.scope.Lookup(x.Name); sym != nil {
		c.info.Types[x] = sym.Type
		return sym.Type
	}

	// Otherwise fall back (undefined names are handled elsewhere if needed).
	return nil
}

func (c *checker) typBinary(x *ast.BinaryExpr) types.T {
	op := x.Op

	switch op {
	case "+", "-", "*", "/", "%", "**":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// --- Ergonomics 4a: implicit str on + ---
		if op == "+" && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}
		if op == "+" && (types.Equal(lt, types.Str) || types.Equal(rt, types.Str)) {
			other := rt
			if types.Equal(lt, types.Str) {
				other = rt
			} else {
				other = lt
			}
			if types.Equal(other, types.Int) || types.Equal(other, types.Float) ||
				types.Equal(other, types.Bool) || types.Equal(other, types.Str) {
				c.info.Types[x] = types.Str
				return types.Str
			}
		}

		// numeric same-type
		if (types.Equal(lt, types.Int) || types.Equal(lt, types.Float)) && types.Equal(lt, rt) {
			c.info.Types[x] = lt
			return lt
		}
		// mixed numeric: int+float or float+int -> float
		if (types.Equal(lt, types.Int) && types.Equal(rt, types.Float)) ||
			(types.Equal(lt, types.Float) && types.Equal(rt, types.Int)) {
			c.info.Types[x] = types.Float
			return types.Float
		}
		c.add(diagAt("DTE0004", x.Span, "invalid operands for '"+op+"'"))
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
		call, ok := x.Rhs.(*ast.CallExpr)
		if !ok {
			c.add(diagAt("DTE0103", x.Span, "pipeline expects a call on the right-hand side"))
			return nil
		}
		id, ok := call.Callee.(*ast.Ident)
		if !ok || id == nil {
			c.add(diagAt("DTE0103", x.Span, "pipeline target must be an identifier"))
			return nil
		}

		lhsT := c.typ(x.Lhs)
		args := make([]types.T, 0, 1+len(call.Args))
		args = append(args, lhsT)
		for _, a := range call.Args {
			args = append(args, c.typ(a))
		}

		set := c.info.Funcs[id.Name]
		if set == nil || len(set.Cands) == 0 {
			c.add(diagAt("DTE0001", id.Span, "pipeline target undefined function: "+id.Name))
			return nil
		}

		// Filter by arity.
		var arityCands []*FuncCand
		for _, cand := range set.Cands {
			if len(cand.Type.Params) == len(args) {
				arityCands = append(arityCands, cand)
			}
		}
		if len(arityCands) == 0 {
			c.add(diagAt("DTE0046", x.Span, "pipeline arity mismatch for call to "+id.Name))
			return nil
		}

		// Exact type match among arity-matching candidates.
		var exact []*FuncCand
	ArgLoop:
		for _, cand := range arityCands {
			for i := range args {
				if !types.Equal(args[i], cand.Type.Params[i]) {
					continue ArgLoop
				}
			}
			exact = append(exact, cand)
		}

		switch len(exact) {
		case 1:
			c.markMovesFromCall(exact[0], call, args)

			ret := exact[0].Type.Ret
			c.info.Types[x] = ret
			return ret
		case 0:
			c.add(diagAt("DTE0101", x.Span, "pipeline has no matching overload for call to "+id.Name))
			return nil
		default:
			c.add(diagAt("DTE0102", x.Span, "pipeline ambiguous overload for call to "+id.Name))
			return nil
		}

	case "<", "<=", ">", ">=", "==", "!=":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		numL := types.Equal(lt, types.Int) || types.Equal(lt, types.Float)
		numR := types.Equal(rt, types.Int) || types.Equal(rt, types.Float)
		if (numL && numR) || types.Equal(lt, rt) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "incomparable operands for '"+op+"'"))
		return nil

	case "and", "or":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if types.Equal(lt, types.Bool) && types.Equal(rt, types.Bool) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "logical operators require bool operands"))
		return nil

	default:
		return nil
	}
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
				c.enforceCallModes(call, cands[0])
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
			c.enforceCallModes(call, cands[0])
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
